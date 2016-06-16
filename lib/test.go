package sequoia

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Test struct {
	Templates map[string][]ActionSpec
	Actions   []ActionSpec
	Flags     TestFlags
	Cm        *ContainerManager
}

type ActionSpec struct {
	Describe    string
	Image       string
	Command     string
	Wait        bool
	Before      string
	Entrypoint  string
	Requires    string
	Concurrency string
	Duration    string
	Save        string
	Repeat      int
	Until       string
	Include     string
	Template    string
	Args        string
}

type TemplateSpec struct {
	Name    string
	Actions []ActionSpec
}

func ActionsFromFile(fileName string) []ActionSpec {
	var actions []ActionSpec
	ReadYamlFile(fileName, &actions)
	return actions
}

func ActionsFromArgs(image string, command string, wait bool) []ActionSpec {
	action := ActionSpec{
		Image:   image,
		Command: command,
		Wait:    wait,
	}
	return []ActionSpec{action}
}

func NewTest(flags TestFlags, cm *ContainerManager) Test {

	// define test actions from config and flags
	var templates = make(map[string][]ActionSpec)
	var actions []ActionSpec
	switch flags.Mode {
	case "image":
		actions = ActionsFromArgs(*flags.ImageName, *flags.ImageCommand, *flags.ImageWait)
	default:
		actions = ActionsFromFile(*flags.TestFile)
	}
	return Test{templates, actions, flags, cm}
}

func (t *Test) Run(scope Scope) {

	// do optional setup
	if *t.Flags.SkipSetup == false {
		// if in default mode purge all containers
		if t.Flags.Mode == "" && *t.Flags.SkipCleanup == false {
			t.Cm.RemoveAllContainers()
			scope.Provider.ProvideCouchbaseServers(scope.Spec.Servers)
		}
		scope.Setup()
	} else if scope.Provider.GetType() != "docker" {
		// non-dynamic IP's need to be extrapolated before test
		scope.Provider.ProvideCouchbaseServers(scope.Spec.Servers)
		scope.InitCli()
	} else {
		// not doing setup but need to get cb versions
		scope.InitCli()
	}

	if *t.Flags.SkipTest == true {
		return
	}

	// run at least <repeat> times or forever if -1
	// run can be terminated if Duration flag set
	repeat := *t.Flags.Repeat
	loops := 0
	duration := *t.Flags.Duration

	if duration > 0 {
		go t.ExitAfterDuration(duration)
		if repeat == 0 {
			repeat = -1 // ensure test runs entire duration
		}
	}

	if repeat == -1 {
		// run forever
		for {
			t.runActions(scope, loops, t.Actions)
			// kill test containers
			scope.Cm.RemoveManagedContainers(*t.Flags.SoftCleanup)
			loops++
		}
	} else {
		repeat++
		for loops = 0; loops < repeat; loops++ {
			t.runActions(scope, loops, t.Actions)
			// kill test containers
			scope.Cm.RemoveManagedContainers(*t.Flags.SoftCleanup)
		}
	}
	t.Cm.TapHandle.AutoPlan()

	// do optional cluster teardown
	if *t.Flags.SkipTeardown == false {
		scope.Teardown()
	}

	// do optional cleanup
	if *t.Flags.SkipCleanup == false {
		t.Cleanup(scope)
	}
}

func (t *Test) runActions(scope Scope, loop int, actions []ActionSpec) {

	var lastAction ActionSpec
	scope.Loops = scope.Loops + loop

	// run all actions in test
	for _, action := range actions {

		if action.Include != "" {
			// include template file
			var spec []TemplateSpec
			ReadYamlFile(action.Include, &spec)
			t.CacheIncludedTemplate(spec)
			continue
		}

		if action.Template != "" {
			// run template actions
			if templateActions, ok := t.Templates[action.Template]; ok {
				templateActions = t.ResolveTemplateActions(scope, action)
				t.runActions(scope, loop, templateActions)
			} else {
				ecolorsay("WARNING template not found: " + action.Template)
			}
			continue
		}

		if action.Image == "" {
			// reuse last action image
			action.Image = lastAction.Image

			// reuse last action requires
			if action.Requires == "" {
				action.Requires = lastAction.Requires
			}
			// reuse last duration
			if action.Duration == "" {
				action.Duration = lastAction.Duration
			}
			// reuse last concurrency
			if action.Concurrency == "" {
				action.Concurrency = lastAction.Concurrency
			}
		}

		// check action requirements
		if action.Requires != "" {
			ok := ParseTemplate(&scope, action.Requires)
			pass, err := strconv.ParseBool(ok)
			logerr(err)
			if pass == false {
				colorsay("skipping due to requirements: " + action.Requires)
				lastAction = action
				continue
			}
		}

		// resolve command
		command := scope.CompileCommand(action.Command)

		// resolve duration and concurrency
		var taskDuration time.Duration = 0
		var taskConcurrency = 0
		var err error
		if action.Duration != "" {
			// parse template if units not found
			if strings.Index(action.Duration, "ns") == -1 {
				action.Duration = fmt.Sprintf("%s%s", ParseTemplate(&scope, action.Duration), "ns")
			}
			taskDuration, err = time.ParseDuration(action.Duration)
			logerr(err)
		}
		if action.Concurrency != "" {
			action.Concurrency = ParseTemplate(&scope, action.Concurrency)
			taskConcurrency, err = strconv.Atoi(action.Concurrency)
			logerr(err)
		}

		if action.Describe == "" { // use command as describe
			action.Describe = fmt.Sprintf("start %s: %s", action.Image, strings.Join(command, " "))
		}

		// compile task
		task := ContainerTask{
			Name:        *t.Flags.ContainerName,
			Describe:    action.Describe,
			Image:       action.Image,
			Command:     command,
			Async:       !action.Wait,
			Duration:    taskDuration,
			Concurrency: taskConcurrency,
			LogLevel:    *t.Flags.LogLevel,
			LogDir:      *t.Flags.LogDir,
			CIDs:        []string{},
		}

		if scope.Provider.GetType() == "docker" {
			task.LinksTo = scope.Provider.(*DockerProvider).GetLinkPairs()
		}
		if action.Entrypoint != "" {
			task.Entrypoint = []string{action.Entrypoint}
		}

		// run task
		if task.Async == true {
			go t.runTask(&scope, &task, &action)
		} else {
			t.runTask(&scope, &task, &action)
		}

		lastAction = action
		time.Sleep(5 * time.Second)
	}

}

func (t *Test) runTask(scope *Scope, task *ContainerTask, action *ActionSpec) {

	actionBefore := action.Before
	repeat := action.Repeat
	rChan := make(chan bool) // repeat chan
	uChan := make(chan bool) // until chan

	// generate save key if not specified
	saveKey := action.Save
	if saveKey == "" {
		saveKey = RandStr(6)
	}

	// if command has 'before' then cannot start processing until ready
	if actionBefore != "" {
		var ready = false
		var err error
		for ready == false {
			before := ParseTemplate(scope, actionBefore)
			ready, err = strconv.ParseBool(before)
			logerr(err)
		}
	}

	if action.Command == "" {
		// noop
		return
	}

	if action.Until != "" {
		// start until watcher
		go t.watchTask(scope, task, saveKey, action.Until, uChan)
	}

	// run once
	cid, echan := t.Cm.Run(task)
	scope.SetVarsKV(saveKey, cid)
	go t.WatchErrorChan(echan, task.Concurrency, scope)

	go t.RepeatTask(scope, cid, repeat, rChan)
	if repeat > 0 {
		// waiting on finite number of repeats
		<-rChan
		t.KillTaskContainers(task)
	}

	if action.Until != "" {
		// waiting for until condition satisfied
		<-uChan
		t.KillTaskContainers(task)
	}

}

func (t *Test) CacheIncludedTemplate(spec []TemplateSpec) {

	for _, template := range spec {
		t.Templates[template.Name] = template.Actions
	}
}

// resolve args from include and cache for referencing
func (t *Test) ResolveTemplateActions(scope Scope, action ActionSpec) []ActionSpec {

	var resolvedActions = []ActionSpec{}
	var cachedActions = t.Templates[action.Template]

	for _, subAction := range cachedActions {
		// replace generics args ie $1, $2 with test values
		args := ParseTemplate(&scope, action.Args)
		allArgs := strings.Split(args, ",")
		for i, arg := range allArgs {
			idx := fmt.Sprintf("$%d", i)
			subAction.Command = strings.Replace(subAction.Command, idx, arg, 1)
		}

		// allow inheritance
		if subAction.Wait == false {
			subAction.Wait = action.Wait
		}
		if subAction.Before == "" {
			subAction.Before = action.Before
		}
		if subAction.Requires == "" {
			subAction.Requires = action.Requires
		}
		if subAction.Concurrency == "" {
			subAction.Concurrency = action.Concurrency
		}
		if subAction.Duration == "" {
			subAction.Duration = action.Duration
		}
		if subAction.Save == "" {
			subAction.Save = action.Save
		}
		if subAction.Repeat == 0 {
			subAction.Repeat = action.Repeat
		}
		if subAction.Until == "" {
			subAction.Until = action.Until
		}
		resolvedActions = append(resolvedActions, subAction)
	}

	return resolvedActions
}

func (t *Test) WatchErrorChan(echan chan error, n int, scope *Scope) {
	if n == 0 {
		n = 1
	}
	for i := 0; i < n; i++ {
		if err := <-echan; err != nil {
			if *t.Flags.CollectOnError == true {
				scope.CollectInfo()
			}

			if *t.Flags.StopOnError == true {
				// print test results
				t.Cm.TapHandle.AutoPlan()
				// exit
				os.Exit(1)
			}
		}
	}
	close(echan)
}

func (t *Test) KillTaskContainers(task *ContainerTask) {
	// until removes task containers when reached
	for _, id := range task.CIDs {
		t.Cm.RemoveContainer(id)
	}
}

func (t *Test) RepeatTask(scope *Scope, cid string, repeat int, done chan bool) {
	// run repeat num times
	for repeat != 0 {
		// only start if it stopped
		if status, err := scope.Cm.GetStatus(cid); err == nil {
			if status == "exited" {
				scope.Cm.StartContainer(cid, nil)
				if repeat > 0 {
					repeat--
				}
			}
		} else {
			// container has been removed
			break
		}
		time.Sleep(1 * time.Second)
	}
	done <- true

}

func (t *Test) watchTask(scope *Scope, task *ContainerTask, saveKey string, condition string, done chan bool) {
	var _done bool
	var err error

	// replace instances of self with savekey
	for _done == false {

		id, ok := scope.GetVarsKV(saveKey)
		if ok == true {
			// make sure we have not been killed by 'duration' or 'repeat' conditions
			if _, err := scope.Cm.GetStatus(id); err != nil {
				break
			}
			condition = strings.Replace(condition, "__self__", saveKey, -1)
			rv := ParseTemplate(scope, condition)
			_done, err = strconv.ParseBool(rv)
			logerr(err)
			time.Sleep(1 * time.Second)
		}
	}
	done <- true
}

func (t *Test) ExitAfterDuration(sec int) {
	// wait
	time.Sleep(time.Duration(sec) * time.Second)
	// print test results
	t.Cm.TapHandle.AutoPlan()
	// exit
	os.Exit(0)
}

func (t *Test) Cleanup(s Scope) {
	soft := *t.Flags.SoftCleanup
	s.Cm.RemoveManagedContainers(soft)
	if s.Provider.GetType() == "docker" {
		// save logs
		if *t.Flags.LogLevel > 0 {
			s.Provider.(*DockerProvider).Cm.SaveContainerLogs(*t.Flags.LogDir)
		}
		s.Provider.(*DockerProvider).Cm.RemoveManagedContainers(soft)
	}
}
