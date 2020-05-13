package app

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sync"

	"github.com/SimonBaeumer/commander/pkg/output"
	"github.com/SimonBaeumer/commander/pkg/runtime"
	"github.com/SimonBaeumer/commander/pkg/suite"
)

// TestCommand executes the test argument
// file is the path to the configuration file
// title ist the title of test which should be executed, if empty it will execute all tests
// ctx holds the command flags
func TestCommand(input string, title string, ctx AddCommandContext) error {
	if ctx.Verbose == true {
		log.SetOutput(os.Stdout)
	}

	if input == "" {
		input = CommanderFile
	}
	inputType, err := os.Stat(input)
	if err != nil {
		return fmt.Errorf("Error " + err.Error())
	}

	var results <-chan runtime.TestResult
	if inputType.IsDir() {
		fmt.Println("Starting test against directory: " + input + "...")
		fmt.Println("")
		// since no handling of file errors occur should we not allow title
		results, err = testDir(input, title)
	} else {
		fmt.Println("Starting test file " + input + "...")
		fmt.Println("")
		results, err = testFile(input, title)
	}

	if err != nil {
		return fmt.Errorf(err.Error())
	}

	out := output.NewCliOutput(!ctx.NoColor)
	if !out.Start(results) {
		return fmt.Errorf("Test suite failed, use --verbose for more detailed output")
	}

	return nil
}

func testDir(directory string, title string) (<-chan runtime.TestResult, error) {
	files, err := ioutil.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("Error " + err.Error())
	}

	results := make(chan runtime.TestResult)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, f := range files {
			// Skip reading dirs for now. Should we also check valid file types?
			if f.IsDir() {
				continue
			}

			fileResults, err := testFile(directory+"/"+f.Name(), title)
			if err != nil {
				panic(fmt.Sprintf("%s: %s", f.Name(), err))
			}

			for r := range fileResults {
				r.FileName = f.Name()
				results <- r
			}
		}
	}()

	go func(ch chan runtime.TestResult) {
		wg.Wait()
		close(results)
	}(results)

	return results, nil
}

func testFile(input string, title string) (<-chan runtime.TestResult, error) {
	content, err := ioutil.ReadFile(input)
	if err != nil {
		return nil, fmt.Errorf("Error " + err.Error())
	}

	var s suite.Suite
	s = suite.ParseYAML(content)
	tests := s.GetTests()
	// Filter tests if test title was given
	if title != "" {
		test, err := s.GetTestByTitle(title)
		if err != nil {
			return nil, err
		}
		tests = []runtime.TestCase{test}
	}

	r := runtime.NewRuntime(s.Nodes...)
	results := r.Start(tests)

	return results, nil
}
