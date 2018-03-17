package foreman

import (
	"reflect"
	"time"

	"google.golang.org/grpc"

	"golang.org/x/net/context"

	"github.com/kelda/kelda/connection"
	"github.com/kelda/kelda/counter"
	"github.com/kelda/kelda/db"
	"github.com/kelda/kelda/minion/pb"

	log "github.com/sirupsen/logrus"
)

// Credentials that the foreman should use to connect to its minions.
var credentials connection.Credentials

type client interface {
	setMinion(pb.MinionConfig) error
	getMinion() (pb.MinionConfig, error)
	Close()
}

type clientImpl struct {
	pb.MinionClient
	cc *grpc.ClientConn
}

var c = counter.New("Foreman")

// Run checks for updates to the machine table and starts and stops minion threads
// in response.
func Run(conn db.Conn, creds connection.Credentials) {
	credentials = creds
	// A map from cloud ID to the stop channel for the corresponding minion thread.
	minionChans := make(map[string]chan struct{})

	for range conn.Trigger(db.MachineTable).C {
		machines := conn.SelectFromMachine(func(m db.Machine) bool {
			return m.PublicIP != "" && m.PrivateIP != "" &&
				m.CloudID != "" && m.Status != db.Stopping
		})
		updateMinions(conn, machines, minionChans)
	}
}

func updateMinions(conn db.Conn, machines []db.Machine,
	minionChans map[string]chan struct{}) {

	seen := make(map[string]struct{})
	for _, m := range machines {
		_, ok := minionChans[m.CloudID]

		if !ok {
			stop := make(chan struct{})
			minionChans[m.CloudID] = stop
			go newMinion(conn, m.CloudID, stop)
		}

		seen[m.CloudID] = struct{}{}
	}

	for id := range minionChans {
		if _, ok := seen[id]; !ok {
			close(minionChans[id])
			delete(minionChans, id)
		}
	}
}

var newMinion = newMinionImpl

func newMinionImpl(conn db.Conn, cloudID string, stop chan struct{}) {
	// Threads that aren't currently connected to their minion should run more often.
	frequentTick := time.NewTicker(5 * time.Second)
	tableTrigger := conn.TriggerTick(60, db.BlueprintTable, db.MachineTable)
	defer frequentTick.Stop()
	defer tableTrigger.Stop()

	// If the other machines in the cluster are not ready by this time, we will
	// configure the cluster as if they were not part of it. This grace period
	// is useful for dealing with race conditions when the cluster is first
	// being connected to. During this time, we don't want to configure etcd
	// with a portion of the masters just to restart it when the rest of the
	// masters finish connecting shortly after.
	waitForMachinesCutoff := time.Now().Add(5 * time.Minute)

	var connected bool
	for {
		select {
		case <-stop:
		case <-frequentTick.C:
			if connected {
				continue
			}
		case <-tableTrigger.C:
		}

		// In a race between a closed stop and a trigger, choose stop.
		select {
		case <-stop:
			return
		default:
		}

		var role pb.MinionConfig_Role
		role, connected = runOnce(waitForMachinesCutoff, conn, cloudID)
		setMinionStatus(conn, cloudID, role, connected)
	}
}

// runOnce attempts to connect to the minion at the machine defined by
// `cloudID`, and update its configuration. It returns the minion's role, and
// connection status. The caller is expected to update the database with this
// information so that the cloud package can reference it.
func runOnce(waitForMachinesCutoff time.Time, conn db.Conn, cloudID string) (
	pb.MinionConfig_Role, bool) {

	var blueprint string
	var machines []db.Machine
	conn.Txn(db.BlueprintTable, db.MachineTable).Run(func(view db.Database) error {
		bp, _ := view.GetBlueprint()
		blueprint = bp.Blueprint.String()

		machines = view.SelectFromMachine(func(m db.Machine) bool {
			return m.CloudID != "" && m.PublicIP != "" &&
				m.PrivateIP != "" && m.Status != db.Stopping
		})
		return nil
	})

	var minionMachine db.Machine
	var found bool
	for _, m := range machines {
		if m.CloudID == cloudID {
			minionMachine = m
			found = true
		}
	}
	if !found {
		log.Debugf("Failed to get machine with ID %s", cloudID)
		return pb.MinionConfig_NONE, false
	}

	cli, err := newClient(minionMachine.PublicIP)
	if err != nil {
		log.WithError(err).Debugf("Failed to connect to minion %s", cloudID)
		return pb.MinionConfig_NONE, false
	}
	defer cli.Close()

	currConfig, err := cli.getMinion()
	if err != nil {
		log.WithError(err).Debug("Failed to get minion config")
		return pb.MinionConfig_NONE, false
	}

	// If there isn't enough information to generate a complete minion config
	// yet, then don't try to set it. However, if enough time has elapsed, then
	// go ahead and use the limited information we have to set the config. This
	// way, if machines fail and never connect, the rest of the cluster can
	// still operate.
	if !clusterReady(machines) && time.Now().Before(waitForMachinesCutoff) {
		return currConfig.Role, true
	}

	newConfig := makeConfig(machines, minionMachine, blueprint)
	if !reflect.DeepEqual(currConfig, newConfig) {
		err = cli.setMinion(newConfig)
		if err != nil {
			log.WithError(err).Debug("Failed to set minion config.")
		}
	}
	return currConfig.Role, true
}

// clusterReady returns whether we have enough information to generate a minion
// config. For example, if there are machines with an unknown role, they might
// cause an etcd restart once they connect since the EtcdMembers will change.
func clusterReady(machines []db.Machine) bool {
	for _, m := range machines {
		if m.Role == db.None || m.PrivateIP == "" {
			return false
		}
	}
	return true
}

func makeConfig(machines []db.Machine, minionMachine db.Machine,
	blueprint string) pb.MinionConfig {

	var etcdIPs []string
	for _, m := range machines {
		if m.Role == db.Master && m.PrivateIP != "" {
			etcdIPs = append(etcdIPs, m.PrivateIP)
		}
	}

	return pb.MinionConfig{
		FloatingIP:     minionMachine.FloatingIP,
		PrivateIP:      minionMachine.PrivateIP,
		Blueprint:      blueprint,
		Provider:       string(minionMachine.Provider),
		Size:           minionMachine.Size,
		Region:         minionMachine.Region,
		EtcdMembers:    etcdIPs,
		AuthorizedKeys: minionMachine.SSHKeys,
	}
}

func setMinionStatus(conn db.Conn, cloudID string, role pb.MinionConfig_Role,
	isConnected bool) {
	conn.Txn(db.MachineTable).Run(func(view db.Database) error {
		rows := view.SelectFromMachine(func(dbm db.Machine) bool {
			return dbm.CloudID == cloudID
		})
		if len(rows) != 1 {
			log.WithField("machine", cloudID).Debug(
				"Failed to find machine in database to update status. " +
					"It was most likely stopped while we were " +
					"querying the machine's status.")
			return nil
		}

		dbm := rows[0]
		dbm.Role = db.PBToRole(role)
		dbm.Connected = isConnected
		if status := db.ConnectionStatus(dbm); status != "" {
			dbm.Status = status
		}
		view.Commit(dbm)
		return nil
	})
}

func newClientImpl(ip string) (client, error) {
	c.Inc("New Minion Client")
	cc, err := connection.Client("tcp", ip+":9999", credentials.ClientOpts())
	if err != nil {
		c.Inc("New Minion Client Error")
		return nil, err
	}

	return clientImpl{pb.NewMinionClient(cc), cc}, nil
}

// Storing in a variable allows us to mock it out for unit tests
var newClient = newClientImpl

func (cl clientImpl) getMinion() (pb.MinionConfig, error) {
	c.Inc("Get Minion")
	ctx, _ := context.WithTimeout(context.Background(), 60*time.Second)
	cfg, err := cl.GetMinionConfig(ctx, &pb.Request{})
	if err != nil {
		c.Inc("Get Minion Error")
		return pb.MinionConfig{}, err
	}

	return *cfg, nil
}

func (cl clientImpl) setMinion(cfg pb.MinionConfig) error {
	c.Inc("Set Minion")
	ctx, _ := context.WithTimeout(context.Background(), 60*time.Second)
	_, err := cl.SetMinionConfig(ctx, &cfg)
	if err != nil {
		c.Inc("Set Minion Error")
	}
	return err
}

func (cl clientImpl) Close() {
	c.Inc("Close Client")
	cl.cc.Close()
}
