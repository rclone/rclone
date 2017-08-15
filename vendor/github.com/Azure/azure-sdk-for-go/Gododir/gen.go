package main

// To run this package...
// go run gen.go -- --sdk 3.14.16

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	do "gopkg.in/godo.v2"
)

type service struct {
	Name        string
	Fullname    string
	Namespace   string
	Packages    []string
	TaskName    string
	Version     string
	Input       string
	Output      string
	Swagger     string
	SubServices []service
	Modeler     modeler
	Extension   extension
}

type modeler string

const (
	swagger     modeler = "Swagger"
	compSwagger modeler = "CompositeSwagger"
)

type extension string

const (
	md   extension = "md"
	json extension = "json"
)

const (
	testsSubDir = "tests"
)

type mapping struct {
	Plane       string
	InputPrefix string
	Services    []service
}

var (
	gopath          = os.Getenv("GOPATH")
	sdkVersion      string
	autorestDir     string
	swaggersDir     string
	testGen         bool
	deps            do.S
	services        = []*service{}
	servicesMapping = []mapping{
		{
			Plane:       "arm",
			InputPrefix: "arm-",
			Services: []service{
				{
					Name:    "advisor",
					Version: "2017-04-19",
				},
				{
					Name:    "analysisservices",
					Version: "2016-05-16",
				},
				{
					Name:    "apimanagement",
					Swagger: "compositeApiManagementClient",
					Modeler: compSwagger,
				},
				{
					Name:    "appinsights",
					Swagger: "compositeAppInsightsManagementClient",
					Modeler: compSwagger,
				},
				{
					Name:    "authorization",
					Version: "2015-07-01",
				},
				{
					Name:    "automation",
					Swagger: "compositeAutomation",
					Modeler: compSwagger,
				},
				{
					Name:    "batch",
					Version: "2017-01-01",
					Swagger: "BatchManagement",
				},
				{
					Name:    "billing",
					Version: "2017-04-24-preview",
				},
				{
					Name:    "cdn",
					Version: "2016-10-02",
				},
				{
					// bug in AutoRest (duplicated files)
					Name: "cognitiveservices",
					// Version: "2017-04-18",
					Version: "2016-02-01-preview",
				},
				{
					Name:    "commerce",
					Version: "2015-06-01-preview",
				},
				{
					Name:    "compute",
					Version: "2016-04-30-preview",
				},
				{
					Name:    "containerservice",
					Version: "2017-01-31",
					Swagger: "containerService",
					Input:   "compute",
				},
				{
					Name:    "consumption",
					Version: "2017-04-24-preview",
				},
				{
					Name:    "containerregistry",
					Version: "2017-03-01",
				},
				{
					Name:    "customer-insights",
					Version: "2017-01-01",
				},
				{
					Name: "datalake-analytics",
					SubServices: []service{
						{
							Name:    "account",
							Version: "2016-11-01",
						},
					},
				},
				{
					Name: "datalake-store",
					SubServices: []service{
						{
							Name:    "account",
							Version: "2016-11-01",
						},
					},
				},
				{
					Name:    "devtestlabs",
					Version: "2016-05-15",
					Swagger: "DTL",
				},
				{
					Name:    "disk",
					Version: "2016-04-30-preview",
					Swagger: "disk",
					Input:   "compute",
				},
				{
					Name:    "dns",
					Version: "2016-04-01",
				},
				{
					Name:    "documentdb",
					Version: "2015-04-08",
				},
				{
					Name:    "eventhub",
					Version: "2015-08-01",
					Swagger: "EventHub",
				},
				{
					Name:    "graphrbac",
					Swagger: "compositeGraphRbacManagementClient",
					Modeler: compSwagger,
				},
				{
					Name:    "hdinsight",
					Swagger: "compositeHDInsight",
					Modeler: compSwagger,
				},
				{
					Name:    "insights",
					Swagger: "compositeInsightsManagementClient",
					Modeler: compSwagger,
				},
				{
					Name:    "intune",
					Version: "2015-01-14-preview",
				},
				{
					Name:    "iothub",
					Version: "2016-02-03",
				},
				{
					Name:    "keyvault",
					Version: "2015-06-01",
				},
				{
					Name:    "logic",
					Version: "2016-06-01",
				},
				{
					Name: "machinelearning",
					SubServices: []service{
						{
							Name:    "commitmentplans",
							Version: "2016-05-01-preview",
							Swagger: "commitmentPlans",
							Input:   "machinelearning",
						},
						{
							Name:    "webservices",
							Version: "2017-01-01",
							Input:   "machinelearning",
						},
					},
				},
				{
					Name:    "mediaservices",
					Version: "2015-10-01",
					Swagger: "media",
				},
				{
					Name:    "mobileengagement",
					Version: "2014-12-01",
					Swagger: "mobile-engagement",
				},
				{
					Name:    "monitor",
					Swagger: "compositeMonitorManagementClient",
					Modeler: compSwagger,
				},
				{
					Name:    "network",
					Swagger: "compositeNetworkClient",
					Modeler: compSwagger,
				},
				{
					Name:    "notificationhubs",
					Version: "2017-04-01",
				},
				{
					// bug in the Go generator https://github.com/Azure/autorest/issues/2219
					Name: "operationalinsights",
					// Swagger: "compositeOperationalInsights",
					// Modeler: compSwagger,
					Version: "2015-11-01-preview",
				},
				{
					Name:    "powerbiembedded",
					Version: "2016-01-29",
				},
				{
					// bug in the go generator
					Name: "recoveryservices",
					// 	Swagger: "compositeRecoveryServicesClient",
					// 	Modeler: compSwagger,
					Version: "2016-06-01",
					Swagger: "vaults",
				},
				{
					// When using the readme.md, there is an exception in the modeler
					Name:    "recoveryservicesbackup",
					Version: "2016-12-01",
					// Swagger:   "readme",
					// Extension: md,
					Swagger: "backupManagement",
				},
				{
					Name:    "recoveryservicessiterecovery",
					Version: "2016-08-10",
					Swagger: "service",
				},
				{
					Name:    "redis",
					Version: "2016-04-01",
				},
				{
					Name:    "relay",
					Version: "2016-07-01",
				},
				{
					Name:    "resourcehealth",
					Version: "2015-01-01",
				},
				{
					Name: "resources",
					SubServices: []service{
						{
							Name:    "features",
							Version: "2015-12-01",
						},
						{
							Name:    "links",
							Version: "2016-09-01",
						},
						{
							Name:    "locks",
							Version: "2016-09-01",
						},
						{
							Name:    "managedapplications",
							Version: "2016-09-01-preview",
						},
						{
							Name:    "policy",
							Version: "2016-12-01",
						},
						{
							Name:    "resources",
							Version: "2016-09-01",
							// Version: "2017-05-10",
						},
						{
							Name:    "subscriptions",
							Version: "2016-06-01",
						},
					},
				},
				{
					Name:    "scheduler",
					Version: "2016-03-01",
				},
				{
					Name:    "search",
					Version: "2015-08-19",
				},
				{
					Name:    "servermanagement",
					Version: "2016-07-01-preview",
				},
				{
					Name:    "service-map",
					Version: "2015-11-01-preview",
					Swagger: "arm-service-map",
				},
				{
					Name:    "servicebus",
					Version: "2015-08-01",
				},
				{
					Name:    "servicefabric",
					Version: "2016-09-01",
				},
				{
					Name:    "sql",
					Swagger: "compositeSql",
					Modeler: compSwagger,
				},
				{
					Name:    "storage",
					Version: "2016-12-01",
				},
				{
					Name:    "storageimportexport",
					Version: "2016-11-01",
				},
				{
					Name:    "storsimple8000series",
					Version: "2017-06-01",
					Swagger: "storsimple",
				},
				{
					Name:    "streamanalytics",
					Swagger: "compositeStreamAnalytics",
					Modeler: compSwagger,
				},
				// {
				// error in the modeler
				// 	Name:    "timeseriesinsights",
				// 	Version: "2017-02-28-preview",
				// },
				{
					Name:    "trafficmanager",
					Version: "2015-11-01",
				},
				{
					Name:    "web",
					Swagger: "compositeWebAppClient",
					Modeler: compSwagger,
				},
			},
		},
		{
			Plane:       "dataplane",
			InputPrefix: "",
			Services: []service{
				// {
				// 	Name:    "batch",
				// 	Version: "2017-01-01.4.0",
				// 	Swagger: "BatchService",
				// },
				// {
				// 	Name:    "insights",
				// 	Swagger: "compositeInsightsClient",
				// 	Modeler: compSwagger,
				// },
				{
					Name:    "keyvault",
					Version: "2016-10-01",
				},
				// {
				// 	Name:    "monitor",
				// 	Swagger: "compositeMonitorClient",
				// 	Modeler: compSwagger,
				// },
				// 	{
				// 		Name: "search",
				// 		SubServices: []service{
				// 			{
				// 				Name:    "searchindex",
				// 				Version: "2016-09-01",
				// 				Input:   "search",
				// 			},
				// 			{
				// 				Name:    "searchservice",
				// 				Version: "2016-09-01",
				// 				Input:   "search",
				// 			},
				// 		},
				// 	},
				// 	{
				// 		Name:    "servicefabric",
				// 		Version: "2016-01-28",
				// 	},
			},
		},
		{
			Plane:       "",
			InputPrefix: "arm-",
			Services: []service{
				{
					Name: "datalake-store",
					SubServices: []service{
						{
							Name:    "filesystem",
							Version: "2016-11-01",
						},
					},
				},
				// {
				// 	Name: "datalake-analytics",
				// 	SubServices: []service{
				// 		{
				// 			Name:    "catalog",
				// 			Version: "2016-11-01",
				// 		},
				// 		{
				// 			Name:    "job",
				// 			Version: "2016-11-01",
				// 		},
				// 	},
				// },
			},
		},
	}
)

func main() {
	for _, swaggerGroup := range servicesMapping {
		swg := swaggerGroup
		for _, service := range swg.Services {
			s := service
			initAndAddService(&s, swg.InputPrefix, swg.Plane)
		}
	}
	do.Godo(tasks)
}

func initAndAddService(service *service, inputPrefix, plane string) {
	if service.Swagger == "" {
		service.Swagger = service.Name
	}
	if service.Extension == "" {
		service.Extension = json
	}
	packages := append(service.Packages, service.Name)
	service.TaskName = fmt.Sprintf("%s>%s", plane, strings.Join(packages, ">"))
	service.Fullname = filepath.Join(plane, strings.Join(packages, string(os.PathSeparator)))
	if service.Modeler == compSwagger {
		service.Input = filepath.Join(inputPrefix+strings.Join(packages, string(os.PathSeparator)), service.Swagger)
	} else {
		input := []string{}
		if service.Input == "" {
			input = append(input, inputPrefix+strings.Join(packages, string(os.PathSeparator)))
		} else {
			input = append(input, inputPrefix+service.Input)
		}
		input = append(input, service.Version)
		if service.Extension == json {
			input = append(input, "swagger")
		}
		input = append(input, service.Swagger)
		service.Input = filepath.Join(input...)
		service.Modeler = swagger
	}
	service.Namespace = filepath.Join("github.com", "Azure", "azure-sdk-for-go", service.Fullname)
	service.Output = filepath.Join(gopath, "src", service.Namespace)

	if service.SubServices != nil {
		for _, subs := range service.SubServices {
			ss := subs
			ss.Packages = append(ss.Packages, service.Name)
			initAndAddService(&ss, inputPrefix, plane)
		}
	} else {
		services = append(services, service)
		deps = append(deps, service.TaskName)
	}
}

func tasks(p *do.Project) {
	p.Task("default", do.S{"setvars", "generate:all", "management"}, nil)
	p.Task("setvars", nil, setVars)
	p.Use("generate", generateTasks)
	p.Use("gofmt", formatTasks)
	p.Use("gobuild", buildTasks)
	p.Use("golint", lintTasks)
	p.Use("govet", vetTasks)
	p.Use("delete", deleteTasks)
	p.Task("management", do.S{"setvars"}, managementVersion)
}

func setVars(c *do.Context) {
	if gopath == "" {
		panic("Gopath not set\n")
	}

	sdkVersion = c.Args.MustString("s", "sdk", "version")
	autorestDir = c.Args.MayString("", "a", "ar", "autorest")
	swaggersDir = c.Args.MayString("C:/", "w", "sw", "swagger")
	testGen = c.Args.MayBool(false, "t", "testgen")
}

func generateTasks(p *do.Project) {
	addTasks(generate, p)
}

func generate(service *service) {
	codegen := "Go"
	if testGen {
		codegen = "Go.TestGen"
		service.Fullname = strings.Join([]string{service.Fullname, testsSubDir}, string(os.PathSeparator))
		service.Output = filepath.Join(service.Output, testsSubDir)
	}

	fmt.Printf("Generating %s...\n\n", service.Fullname)

	delete(service)

	execCommand := "autorest"
	commandArgs := []string{
		"-Input", filepath.Join(swaggersDir, "azure-rest-api-specs", service.Input+"."+string(service.Extension)),
		"-CodeGenerator", codegen,
		"-Header", "MICROSOFT_APACHE",
		"-Namespace", service.Name,
		"-OutputDirectory", service.Output,
		"-Modeler", string(service.Modeler),
		"-PackageVersion", sdkVersion,
	}
	if testGen {
		commandArgs = append([]string{"-LEGACY"}, commandArgs...)
	}

	// default to the current directory
	workingDir := ""

	if autorestDir != "" {
		// if an AutoRest directory was specified then assume
		// the caller wants to use a locally-built version.
		execCommand = "gulp"
		commandArgs = append([]string{"autorest"}, commandArgs...)
		workingDir = filepath.Join(autorestDir, "autorest")
	}

	autorest := exec.Command(execCommand, commandArgs...)
	autorest.Dir = workingDir

	if err := runner(autorest); err != nil {
		panic(fmt.Errorf("Autorest error: %s", err))
	}

	format(service)
	build(service)
	lint(service)
	vet(service)
}

func deleteTasks(p *do.Project) {
	addTasks(format, p)
}

func delete(service *service) {
	fmt.Printf("Deleting %s...\n\n", service.Fullname)
	err := os.RemoveAll(service.Output)
	if err != nil {
		panic(fmt.Sprintf("Error deleting %s : %s\n", service.Output, err))
	}
}

func formatTasks(p *do.Project) {
	addTasks(format, p)
}

func format(service *service) {
	fmt.Printf("Formatting %s...\n\n", service.Fullname)
	gofmt := exec.Command("gofmt", "-w", service.Output)
	err := runner(gofmt)
	if err != nil {
		panic(fmt.Errorf("gofmt error: %s", err))
	}
}

func buildTasks(p *do.Project) {
	addTasks(build, p)
}

func build(service *service) {
	fmt.Printf("Building %s...\n\n", service.Fullname)
	gobuild := exec.Command("go", "build", service.Namespace)
	err := runner(gobuild)
	if err != nil {
		panic(fmt.Errorf("go build error: %s", err))
	}
}

func lintTasks(p *do.Project) {
	addTasks(lint, p)
}

func lint(service *service) {
	fmt.Printf("Linting %s...\n\n", service.Fullname)
	golint := exec.Command(filepath.Join(gopath, "bin", "golint"), service.Namespace)
	err := runner(golint)
	if err != nil {
		panic(fmt.Errorf("golint error: %s", err))
	}
}

func vetTasks(p *do.Project) {
	addTasks(vet, p)
}

func vet(service *service) {
	fmt.Printf("Vetting %s...\n\n", service.Fullname)
	govet := exec.Command("go", "vet", service.Namespace)
	err := runner(govet)
	if err != nil {
		panic(fmt.Errorf("go vet error: %s", err))
	}
}

func managementVersion(c *do.Context) {
	version("management")
}

func version(packageName string) {
	versionFile := filepath.Join(packageName, "version.go")
	os.Remove(versionFile)
	template := `package %s

var (
	sdkVersion = "%s"
)
`
	data := []byte(fmt.Sprintf(template, packageName, sdkVersion))
	ioutil.WriteFile(versionFile, data, 0644)
}

func addTasks(fn func(*service), p *do.Project) {
	for _, service := range services {
		s := service
		p.Task(s.TaskName, nil, func(c *do.Context) {
			fn(s)
		})
	}
	p.Task("all", deps, nil)
}

func runner(cmd *exec.Cmd) error {
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if stdout.Len() > 0 {
		fmt.Println(stdout.String())
	}
	if stderr.Len() > 0 {
		fmt.Println(stderr.String())
	}
	return err
}
