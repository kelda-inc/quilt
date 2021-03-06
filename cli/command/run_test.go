package command

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/fatih/color"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	clientMock "github.com/kelda/kelda/api/client/mocks"
	"github.com/kelda/kelda/blueprint"
	"github.com/kelda/kelda/db"
	"github.com/kelda/kelda/util"
)

type diffTest struct {
	curr, new, exp string
}

func TestDeploymentDiff(t *testing.T) {
	t.Parallel()

	tests := []diffTest{
		{
			curr: "{}",
			new:  "{}",
			exp:  "",
		},
		{
			curr: `{"Machines":[{"Provider":"Amazon"}]}`,
			new:  `{"Machines":[]}`,
			exp: `--- Current
+++ Proposed
@@ -1,7 +1,3 @@
 {
-	"Machines": [
-		{
-			"Provider": "Amazon"
-		}
-	]
+	"Machines": []
 }
`,
		},
		{
			curr: `{"Machines":[{"Provider":"Amazon"},` +
				`{"Provider":"Google"}]}`,
			new: `{"Machines":[{"Provider":"Google"}]}`,
			exp: `--- Current
+++ Proposed
@@ -1,8 +1,5 @@
 {
 	"Machines": [
-		{
-			"Provider": "Amazon"
-		},
 		{
 			"Provider": "Google"
 		}
`,
		},
		{
			curr: `{"Machines":[{"Provider":"Amazon"},` +
				`{"Provider":"Google"}]}`,
			new: `{"Machines":[{"Provider":"Vagrant"}]}`,
			exp: `--- Current
+++ Proposed
@@ -1,10 +1,7 @@
 {
 	"Machines": [
 		{
-			"Provider": "Amazon"
-		},
-		{
-			"Provider": "Google"
+			"Provider": "Vagrant"
 		}
 	]
 }
`,
		},
	}

	for _, test := range tests {
		diff, err := diffDeployment(test.curr, test.new)
		assert.Nil(t, err)
		assert.Equal(t, test.exp, diff)
	}
}

type colorizeTest struct {
	original string
	exp      string
}

func TestColorize(t *testing.T) {
	green := "\x1b[32m"
	red := "\x1b[31m"
	// a reset sequence is inserted after a colorized line
	reset := "\x1b[0m"
	// force colored output for testing
	color.NoColor = false
	tests := []colorizeTest{
		{
			original: "{}",
			exp:      "{}",
		},
		{
			original: "no color\n" +
				"-\tred\n" +
				"+\tgreen\n",
			exp: "no color\n" +
				red + "-\tred\n" + reset +
				green + "+\tgreen\n" + reset,
		},
		{
			original: "\n",
			exp:      "\n",
		},
		{
			original: "\na\n\n",
			exp:      "\na\n\n",
		},
		{
			original: "+----+---+\n",
			exp:      green + "+----+---+\n" + reset,
		},
	}
	for _, test := range tests {
		colorized := colorizeDiff(test.original)
		assert.Equal(t, test.exp, colorized)
	}
}

type confirmTest struct {
	inputs []string
	exp    bool
}

func TestConfirm(t *testing.T) {
	tests := []confirmTest{
		{
			inputs: []string{"y"},
			exp:    true,
		},
		{
			inputs: []string{"yes"},
			exp:    true,
		},
		{
			inputs: []string{"YES"},
			exp:    true,
		},
		{
			inputs: []string{"n"},
			exp:    false,
		},
		{
			inputs: []string{"no"},
			exp:    false,
		},
		{
			inputs: []string{"foo", "no"},
			exp:    false,
		},
		{
			inputs: []string{"foo", "no", "yes"},
			exp:    false,
		},
	}
	for _, test := range tests {
		in := bytes.NewBufferString(strings.Join(test.inputs, "\n"))
		res, err := confirm(in, "")
		assert.Nil(t, err)
		assert.Equal(t, test.exp, res)
	}
}

func TestPromptsUser(t *testing.T) {
	oldConfirm := confirm
	defer func() {
		confirm = oldConfirm
	}()

	compile = func(path string, args []string) (blueprint.Blueprint, error) {
		return blueprint.Blueprint{}, nil
	}

	util.AppFs = afero.NewMemMapFs()
	for _, confirmResp := range []bool{true, false} {
		confirm = func(in io.Reader, prompt string) (bool, error) {
			return confirmResp, nil
		}

		c := new(clientMock.Client)
		c.On("QueryBlueprints").Return([]db.Blueprint{{
			Blueprint: blueprint.Blueprint{Namespace: "old"},
		}}, nil)
		c.On("Deploy", "{}").Return(nil)

		util.WriteFile("test.js", []byte(""), 0644)
		runCmd := &Run{
			connectionHelper: connectionHelper{client: c},
			blueprint:        "test.js",
		}
		runCmd.Run()

		if confirmResp {
			c.AssertCalled(t, "Deploy", mock.Anything)
		} else {
			c.AssertNotCalled(t, "Deploy", mock.Anything)
		}
	}
}

func TestRunFlags(t *testing.T) {
	t.Parallel()

	expBlueprint := "blueprint"
	checkRunParsing(t, []string{"-blueprint", expBlueprint},
		Run{blueprint: expBlueprint, blueprintArgs: []string{}}, nil)
	checkRunParsing(t, []string{expBlueprint}, Run{blueprint: expBlueprint,
		blueprintArgs: []string{}}, nil)
	checkRunParsing(t, []string{"-f", expBlueprint},
		Run{force: true, blueprint: expBlueprint,
			blueprintArgs: []string{}}, nil)
	checkRunParsing(t, []string{}, Run{}, errors.New("no blueprint specified"))
}

func TestRunArgs(t *testing.T) {
	t.Parallel()

	expBlueprint := "blueprint"
	checkRunParsing(t, []string{expBlueprint, "hello", "world"},
		Run{blueprint: expBlueprint,
			blueprintArgs: []string{"hello", "world"}}, nil)
	checkRunParsing(t, []string{"-blueprint", expBlueprint, "hello", "world"},
		Run{blueprint: expBlueprint,
			blueprintArgs: []string{"hello", "world"}}, nil)
	checkRunParsing(t, []string{"-f", "-blueprint", expBlueprint, "hello", "world"},
		Run{force: true, blueprint: expBlueprint,
			blueprintArgs: []string{"hello", "world"}}, nil)
}

func checkRunParsing(t *testing.T, args []string, expFlags Run, expErr error) {
	runCmd := NewRunCommand()
	err := parseHelper(runCmd, args)

	if expErr != nil {
		if err.Error() != expErr.Error() {
			t.Errorf("Expected error %s, but got %s",
				expErr.Error(), err.Error())
		}
		return
	}

	assert.Nil(t, err)
	assert.Equal(t, expFlags.blueprint, runCmd.blueprint)
	assert.Equal(t, expFlags.blueprintArgs, runCmd.blueprintArgs)
	assert.Equal(t, expFlags.force, runCmd.force)
}
