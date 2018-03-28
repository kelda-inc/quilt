Kelda Change Log
================

Up Next
-------------

Release 0.13.0
-------------

- Replace the custom Kelda scheduler for managing containers with Kubernetes.
- Replace the `kelda init` command with the new commands
`kelda configure-provider` and `kelda base-infrastructure`.

Release 0.12.0
-------------

- Allow containers to run in `privileged` mode by setting the `privileged` flag
in the Container object. This is necessary for containers to access devices on
the host machine, such as `/dev/fuse`.
- Allow containers to specify `hostPath` volumes (also known as Bind volumes in
the Docker documentation. `hostPath` is the Kubernetes equivalent.). This is
necessary for containers to access the Docker run socket on the host. Note that
`hostPath` volumes are specific to the host for which the container is scheduled,
and the data in the volume will not be transferred if the container is
rescheduled to another machine. It's therefore insufficient, and possibly
incorrect, to use a `hostPath` volume to store hard-state, such as a database.

Release 0.11.0
-------------

- Only allow object literal arguments to the Container, Image, LoadBalancer,
and Infrastructure constructors.
- Remove the setEnv() and withEnv() Container methods. Users should instead set
environment variables through the constructor call or by directly accessing the
.env attribute of the Container instance.
- Remove the deprecated LoadBalancer.hostname() method in favor of
LoadBalancer.getHostname().

Release 0.10.0
-------------

- Remove the `.q` TLD for Kelda hostnames. Kubernetes doesn't support hostnames
with TLDs.
- The Container, Infrastructure, LoadBalancer, and Image constructors can now
receive arguments as a single object -- e.g. `new Container({ name: 'name:, image:
'image' })`. The old way of passing arguments is deprecated.
- LoadBalancer.hostname() has been deprecated in favor of LoadBalancer.getHostname().

Release 0.9.0
-------------

- Container and Load Balancer hostnames must now comply with the Kubernetes
DNS_LABEL spec. Specifically, hostnames must contain only lower case
characters, numbers, or hyphens.
- `kelda.hostIP` has been removed. It is no longer possible to insert a
container's public IP into its environment variables. Containers that need
this feature should query their public IP at runtime using a service such as
checkip.amazon.com, or pass containers a floating IP (or DNS name backed by a
floating IP) and use placement rules to ensure the container is scheduled at
that floating IP.

Javascript API changes:
- Remove deprecated methods `allowFrom` and `allow`.

Release 0.8.0
-------------

JavaScript API changes:
- Add `allowTraffic` method to allow network traffic and connections between two
sets of `Connectable`s (i.e. `publicInternet`, `LoadBalancer`, and `Container`).
- Deprecate `Connectable.allowFrom()` method in favor of `allowTraffic`.

Release 0.7.0
-------------

- Properly set the container's hostname. Before, a container querying its own
hostname would get its Docker ID, which is meaningless in the Kelda network.
- Only allow a single base infrastructure. We had no good use case for having
multiple base infrastructures, and the feature added unnecessary complexity.
- Allow blueprints to accept command line arguments.
- Allow configuring the Kelda daemon host socket with the `KELDA_HOST`
environment variable. While using the default socket is sufficient for most
scenarios, using a non-default socket path is useful for connecting to a remote
daemon, or running multiple daemons on the same machine. Setting the socket
path using an environment variable allows users to simply set the environment
variable once, and not worry about setting the correct flags in all `kelda`
commands.

Release 0.6.0
-------------

Release 0.6.0 simplifies the `kelda ssh` command and allows containers to reference
their public IP.

- The Kelda command line utility now accepts container names in addition to
IDs.  For example, `kelda ssh spark-master` can be used instead of
`kelda ssh 6cf081531346`.
- Allow containers to reference their public IP via an environment variable or
file. For example, to create a container with its public IP in SPARK_PUBLIC_DNS:
```
new kelda.Container('spark', 'keldaio/spark', {
  env: {
    SPARK_PUBLIC_DNS: kelda.hostIP,
  },
})
```

Release 0.5.0
-------------

Kelda, formerly Quilt, has a brand new name!

Release 0.5.0 includes two major changes: secret support, and a new `kelda
show` format for listing machines. The new secret feature makes storing and using
sensitive configuration values a first-level concept of Kelda -- the secrets
are encrypted at rest and in transit, and access to them is limited based on
least privilege. `kelda show` now displays the machines that are currently
running in the cloud, rather than the machines desired by the user. This way,
if a user removes a machine from a blueprint, `kelda show` will show the
machine until it is actually removed from the cloud.

- Kelda, formerly Quilt, has a brand new name!
- Fix a bug where `quilt setup-tls` would fail when writing to a directory whose
parent does not exist.
- Auto-generate TLS credentials when starting the daemon if the credentials in
`~/.quilt/tls` don't already exist.
- Always encrypt communication with the daemon.
- Try using the Kelda-managed SSH key when connecting to machines. The
Kelda-managed SSH key should work most of the time because the
daemon automatically grants it access to the cluster.
- Fix a bug where floating IPs would not get properly assigned in GCE.
- Add Infrastructure class for deploying Kelda machines. createDeployment()
and the Deployment class are now deprecated, and users should transition
to using Infrastructure instead.
- Add support for Secrets -- a Kelda-blessed method for securely using
sensitive values. Before, secrets (such as OAuth tokens) were stored in the
blueprint source. It was very easy to accidentally commit these secrets, and
they had no special protection once deployed to the cluster. Secrets are now a
first-level concept of Kelda. They are encrypted at rest and in transit, and
access to them is limited based on least privilege.
- Fix a race condition TLS credentials would not get installed on the minion, so
it would never enter the "connected" state.

JavaScript API-breaking changes:
- Remove the Container.replicate() method. Users should create multiple
containers by looping.
- Remove the Container.withFiles() method.  Users should use the
'filepathToContent' optional argument when creating a container instead.
- Remove the createDeployment() function, the Machine.asMaster(),
Machine.asWorker(), Machine.deploy(), Machine.asWorker() methods, and
the Deployment class. Users should use the Infrastructure class instead
to create an infrastructure and deploy machines.


Release 0.4.0
-------------

Release 0.4.0 makes some minor UX improvements.

- Check for unexpected keys in the optional argument passed to the Machine,
Container, and Deployment constructors.
- Rename StitchID to BlueprintID in the database. This is an internal
API-breaking change (it changes the API between internal Quilt components).
- Fix TLS encryption for GRPC connections to machines that use floating IPs.
- Support Node.js version 6 (previously, we had some code that failed unless
users were running version 7 or later).

Release 0.3.0
-------------

Release 0.3.0 changes the way that containers are deployed. `Container`s can now
be deployed directly without wrapping them in a `LoadBalancer` (previously known
as `Service`). Many of the methods previously defined on `Service` (such as
`allowFrom` and `placeOn`) have been migrated to `Container` methods.

- Don't use the image cache on the Quilt master when building custom
Dockerfiles. This is necessary to fetch updates when Dockerfiles are
non-deterministic and rely on pulling data from the network.
- Use the latest stable release of Docker Engine.
- Fixed a bug where `quilt inspect` would panic when given a relative path.
- Use the latest release of OVS (2.7.2).
- Remove support for invariants.
- Remove support for placement based on service groups.
- Simplify machine-service placement. For example, deploying a service to a
floating IP is now expressed as `myService.placeOn({floatingIp: '8.8.8.8'})`.
- Remove `Service.connect`. Only `Service.allowFrom` can be used from now on.
- Restart containers if their hostname changes.
- Fix a bug where containers might get assigned duplicate hostnames.
- Remove `Service.children`. Container hostnames should be used from now on.
- Change the container constructor syntax to take optional settings as the
last argument:
```javascript
new Container('imageName', {
  command: ['command', 'args'],
  env: { key: 'val' },
  filepathToContent: { path: 'content' },
});
```
- Require a hostname to be provided to the container constructor:
```javascript
new Container('hostname', 'imageName');
```
- Hostnames are now immutable after the container is constructed -- the
`Container.setHostname` method has been removed.
- Containers can now be `deploy`ed directly without wrapping them in a Service.
Deploying a Service does _not_ deploy the Containers behind it -- the Containers
must be explicitly deployed.

API Breaking Changes:
- Make `placeOn` a method of `Container` rather than `Service`.
- Allow containers to explicitly connect to each other (rather than requiring
all connections to occur by connecting services).
- Change Service.allowFrom so that it allows connections to the load balancer,
and not directly to the containers that get load balanced over.
- Describe services in terms of hostnames rather than container IDs.
- Rename `Service` to `LoadBalancer`.

Release 0.2.0
-------------

Release 0.2.0 introduces two big features: load balancing, and TLS-encrypted
communication for Quilt control traffic.

To use load balancing, simply create and deploy a `Service` -- the hostname
associated with that `Service` will now automatically load balance traffic
across its containers.

TLS is currently optional. If the `tls-dir` flag is omitted, Quilt control
traffic will remain insecure as before.

To enable TLS, run `quilt setup-tls ~/.quilt/tls`, and then start the daemon
with `quilt daemon -tls-dir ~/.quilt/tls` (you can place the TLS certificates
in a different directory if you'd like -- just make sure that the same
directory is use for `setup-tls` and `daemon`). After the daemon starts, all
the subcommands will work as before.

What's new:

- Package the OVS kernel module for the latest DigitalOcean image to speed up
boot times.
- Renamed specs to blueprints.
- Load balancing.
- Upgraded to the latest Docker engine version (17.05.0).
- Fixed bug in Google provider that caused ACLs to be repeatedly added.
- Fixed inbound connections on the Vagrant provider. In other words,
`myService.allowFrom(publicInternet, aPort)` now works.
- Only allocate one Google network per namespace, rather than one network for
each region within a namespace.
- Implement debugging counters accessible through `quilt counters`.
- Disallow IP allocation in subnets governed by routes in the host network. This
fixes a bug where containers would sometimes fail to resolve DNS on DigitalOcean.
- Fixed a bug where etcd would sometimes restart when the daemon connected to machines
that had already been booted. This most visibly resulted in containers restarting.
- Use an exponential backoff algorithm when waiting for cloud provider actions
to complete. This decreases the number of cloud provider API calls done by Quilt.
- `quilt ps` is now renamed to `quilt show`, though the original `quilt ps`
  still works as an alias to `quilt show`.
- `quilt show` now displays the image building status of custom Dockerfiles.
- Let blueprints write to stdout. Before, if blueprints used `console.log`, the
text printed to stdout would break the deployment object.
- `quilt show` now has more status options for machines (booting, connecting,
connected, and reconnecting).
- Allow an admin SSH key access to all machines deployed by the daemon. The key is
specified using the `admin-ssh-private-key` flag to the daemon.
- Support for TLS-encrypted communication between Quilt clients and servers.

Release 0.1.0
-------------

Release 0.1.0 most notably modifies `quilt run` to evaluate Quilt specs using
Node.js, rather than within a Javascript implementation written in Go. This
enables users to make use of many great Node features, such as package management,
versioning, unit testing, a rich ecosystem of modules, and `use
strict`. In order to facilitate this, we now require `node` version 7.10.0 or
greater as a dependency to `quilt run`.

What's new:

- Fix a bug where Amazon spot requests would get cancelled when there are
multiple Quilt daemons running in the same Amazon account.
- Improve the error message for misconfigured Amazon credentials.
- Fix a bug where inbound and outbound public traffic would get randomly
dropped.
- Support floating IP assignment in DigitalOcean.
- Support arbitrary GCE projects.
- Upgrade to OVS2.7.
- Fix a race condition where the minion boots before OVS is ready.
- Build the OVS kernel module at runtime if a pre-built version is not
available.
- Evaluate specs using Node.js.

Release 0.0.1
-------------

Release 0.0.1 is an experimental release targeted at students in the CS61B
class at UC Berkeley.

Release 0.0.0
-------------

We are proud to announce the initial release of [Kelda](http://kelda.io)!  This
release provides an alpha quality implementation which can deploy a whole [host
of distributed applications](http://github.com/kelda) to Amazon EC2, Google
Cloud Engine, or DigitalOcean.  We're excited to begin this journey with our
inaugural release!  Please try it out and [let us
know](http://kelda.io/#contact) what you think.
