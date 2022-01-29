// cmdtest_test creates a testable interface to rclone main
//
// The interface is used to perform end-to-end test of
// commands, flags, environment variables etc.

package cmdtest

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain is initially called by go test to initiate the testing.
// TestMain is also called during the tests to start rclone main in a fresh context (using exec.Command).
// The context is determined by setting/finding the environment variable RCLONE_TEST_MAIN
func TestMain(m *testing.M) {
	_, found := os.LookupEnv(rcloneTestMain)
	if !found {
		// started by Go test => execute tests
		err := os.Setenv(rcloneTestMain, "true")
		if err != nil {
			log.Fatalf("Unable to set %s: %s", rcloneTestMain, err.Error())
		}
		os.Exit(m.Run())
	} else {
		// started by func rcloneExecMain => call rclone main in cmdtest.go
		err := os.Unsetenv(rcloneTestMain)
		if err != nil {
			log.Fatalf("Unable to unset %s: %s", rcloneTestMain, err.Error())
		}
		main()
	}
}

const rcloneTestMain = "RCLONE_TEST_MAIN"

// rcloneExecMain calls rclone with the given environment and arguments.
// The environment variables are in a single string separated by ;
// The terminal output is retuned as a string.
func rcloneExecMain(env string, args ...string) (string, error) {
	_, found := os.LookupEnv(rcloneTestMain)
	if !found {
		log.Fatalf("Unexpected execution path: %s is missing.", rcloneTestMain)
	}
	// make a call to self to execute rclone main in a predefined environment (enters TestMain above)
	command := exec.Command(os.Args[0], args...)
	command.Env = getEnvInitial()
	if env != "" {
		command.Env = append(command.Env, strings.Split(env, ";")...)
	}
	out, err := command.CombinedOutput()
	return string(out), err
}

// rcloneEnv calls rclone with the given environment and arguments.
// The environment variables are in a single string separated by ;
// The test config file is automatically configured in RCLONE_CONFIG.
// The terminal output is retuned as a string.
func rcloneEnv(env string, args ...string) (string, error) {
	envConfig := env
	if testConfig != "" {
		if envConfig != "" {
			envConfig += ";"
		}
		envConfig += "RCLONE_CONFIG=" + testConfig
	}
	return rcloneExecMain(envConfig, args...)
}

// rclone calls rclone with the given arguments, E.g. "version","--help".
// The test config file is automatically configured in RCLONE_CONFIG.
// The terminal output is retuned as a string.
func rclone(args ...string) (string, error) {
	return rcloneEnv("", args...)
}

// getEnvInitial returns the os environment variables cleaned for RCLONE_ vars (except RCLONE_TEST_MAIN).
func getEnvInitial() []string {
	if envInitial == nil {
		// Set initial environment variables
		osEnv := os.Environ()
		for i := range osEnv {
			if !strings.HasPrefix(osEnv[i], "RCLONE_") || strings.HasPrefix(osEnv[i], rcloneTestMain) {
				envInitial = append(envInitial, osEnv[i])
			}
		}
	}
	return envInitial
}

var envInitial []string

// createTestEnvironment creates a temporary testFolder and
// sets testConfig to testFolder/rclone.config.
func createTestEnvironment(t *testing.T) {
	//Set temporary folder for config and test data
	tempFolder := t.TempDir()
	testFolder = filepath.ToSlash(tempFolder)

	// Set path to temporary config file
	testConfig = testFolder + "/rclone.config"
}

var testFolder string
var testConfig string

// removeTestEnvironment removes the test environment created by createTestEnvironment
func removeTestEnvironment(t *testing.T) {
	// Remove temporary folder with all contents
	err := os.RemoveAll(testFolder)
	require.NoError(t, err)
}

// createTestFile creates the file testFolder/name
func createTestFile(name string, t *testing.T) string {
	err := ioutil.WriteFile(testFolder+"/"+name, []byte("content_of_"+name), 0666)
	require.NoError(t, err)
	return testFolder + "/" + name
}

// createTestFolder creates the folder testFolder/name
func createTestFolder(name string, t *testing.T) string {
	err := os.Mkdir(testFolder+"/"+name, 0777)
	require.NoError(t, err)
	return testFolder + "/" + name
}

// createSimpleTestData creates simple test data in testFolder/subFolder
func createSimpleTestData(t *testing.T) string {
	createTestFolder("testdata", t)
	createTestFile("testdata/file1.txt", t)
	createTestFile("testdata/file2.txt", t)
	createTestFolder("testdata/folderA", t)
	createTestFile("testdata/folderA/fileA1.txt", t)
	createTestFile("testdata/folderA/fileA2.txt", t)
	createTestFolder("testdata/folderA/folderAA", t)
	createTestFile("testdata/folderA/folderAA/fileAA1.txt", t)
	createTestFile("testdata/folderA/folderAA/fileAA2.txt", t)
	createTestFolder("testdata/folderB", t)
	createTestFile("testdata/folderB/fileB1.txt", t)
	createTestFile("testdata/folderB/fileB2.txt", t)
	return testFolder + "/testdata"
}

// removeSimpleTestData removes the test data created by createSimpleTestData
func removeSimpleTestData(t *testing.T) {
	err := os.RemoveAll(testFolder + "/testdata")
	require.NoError(t, err)
}

// TestCmdTest demonstrates and verifies the test functions for end-to-end testing of rclone
func TestCmdTest(t *testing.T) {
	createTestEnvironment(t)
	defer removeTestEnvironment(t)

	// Test simple call and output from rclone
	out, err := rclone("version")
	t.Logf("rclone version\n" + out)
	if assert.NoError(t, err) {
		assert.Contains(t, out, "rclone v")
		assert.Contains(t, out, "version: ")
		assert.NotContains(t, out, "Error:")
		assert.NotContains(t, out, "--help")
		assert.NotContains(t, out, " DEBUG : ")
		assert.Regexp(t, "rclone\\s+v\\d+\\.\\d+", out) // rclone v_.__
	}

	// Test multiple arguments and DEBUG output
	out, err = rclone("version", "-vv")
	if assert.NoError(t, err) {
		assert.Contains(t, out, "rclone v")
		assert.Contains(t, out, " DEBUG : ")
	}

	// Test error and error output
	out, err = rclone("version", "--provoke-an-error")
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "exit status 1")
		assert.Contains(t, out, "Error: unknown flag")
	}

	// Test effect of environment variable
	env := "RCLONE_LOG_LEVEL=DEBUG"
	out, err = rcloneEnv(env, "version")
	if assert.NoError(t, err) {
		assert.Contains(t, out, "rclone v")
		assert.Contains(t, out, " DEBUG : ")
	}

	// Test effect of multiple environment variables, including one with ,
	env = "RCLONE_LOG_LEVEL=DEBUG;RCLONE_LOG_FORMAT=date,shortfile;RCLONE_STATS=173ms"
	out, err = rcloneEnv(env, "version")
	if assert.NoError(t, err) {
		assert.Contains(t, out, "rclone v")
		assert.Contains(t, out, " DEBUG : ")
		assert.Regexp(t, "[^\\s]+\\.go:\\d+:", out) // ___.go:__:
		assert.Contains(t, out, "173ms")
	}

	// Test setup of config file
	out, err = rclone("config", "create", "myLocal", "local")
	if assert.NoError(t, err) {
		assert.Contains(t, out, "[myLocal]")
		assert.Contains(t, out, "type = local")
	}

	// Test creation of simple test data
	createSimpleTestData(t)
	defer removeSimpleTestData(t)

	// Test access to config file and simple test data
	out, err = rclone("lsl", "myLocal:"+testFolder)
	t.Logf("rclone lsl myLocal:testFolder\n" + out)
	if assert.NoError(t, err) {
		assert.Contains(t, out, "rclone.config")
		assert.Contains(t, out, "testdata/folderA/fileA1.txt")
	}

}
