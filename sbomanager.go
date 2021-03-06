package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"strings"
)

const (
	baseUrl = "http://slackbuilds.org/slackbuilds/"
	tmpDir  = "/tmp/sbomanager/"
)

var (
	pkgList    map[string]Package
	arch       string
	version    string
	pkgListUrl string
)

func init() {
	versionFile, err := ioutil.ReadFile("/etc/slackware-version")
	check(err)

	version = strings.Split(strings.Trim(string(versionFile), "\n"), " ")[1]

	pkgListUrl = baseUrl + version + "/SLACKBUILDS.TXT"

	cmd := exec.Command("uname", "-m")
	out, err := cmd.Output()
	arch = strings.Trim(string(out), "\n")

}

func main() {
	flag.Usage = usage
	flag.Parse()
	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(2)
	} else {
		runCommand(flag.Arg(0))
	}
}

func usage() {
	fmt.Println(`Usage:
		aaaa`)
}

func runCommand(cmd string) {
	switch cmd {
	case "search":
		search()
	case "update":
		update()
	case "show":
		show()
	case "install":
		install()
	}
}

func install() {
	loadPkgList()

	instFS := flag.NewFlagSet("install", flag.ExitOnError)
	instFS.Parse(flag.Args())
	keyword := instFS.Arg(1)

	if pkgList[keyword].Name == "" {
		fmt.Println("Package not found")
	} else {
		pkg := pkgList[keyword]
		if pkg.IsInstalled() {
			fmt.Println("Packaged already installed. Use the upgrade command.")
		} else {
			buildList := genBuildList(pkgList[keyword])
			fmt.Println("Building the following packages:")
			fmt.Println(buildList)
			fmt.Printf("(Y/n)")
			c := ""
			fmt.Scanln(&c)
			if strings.ToLower(c) == "y" || c == "\n" {
				for _, pkgName := range buildList {
					installPkgs(pkgName)
				}
			}
		}
	}
}

func loadPkgList() {
	sbomngDir := userHomeDir() + "/.sbomanager/"
	pkgListFileName := sbomngDir + "pkglist"

	pkgListByte, err := ioutil.ReadFile(pkgListFileName)
	check(err)

	err = json.Unmarshal(pkgListByte, &pkgList)
	check(err)

}

func update() {
	fmt.Println("Updating package list...")
	pkgList = genPkgList()
	sbomngDir := userHomeDir() + "/.sbomanager"
	if sbDirExists, err := exists(sbomngDir); !sbDirExists && err == nil {
		err := os.Mkdir(sbomngDir, os.ModeDir|0755)
		check(err)
	}
	pkgListFileName := sbomngDir + "/pkglist"
	out, err := json.Marshal(pkgList)
	err = ioutil.WriteFile(pkgListFileName, out, 0644)
	check(err)
	fmt.Println("Package list updated")
}

func show() {
	loadPkgList()
	showFS := flag.NewFlagSet("show", flag.ExitOnError)
	showFS.Parse(flag.Args())
	keyword := showFS.Arg(1)
	fmt.Printf("Information for package: %s\n\n", keyword)
	if pkgList[keyword].Name == "" {
		fmt.Println("Package not found")
	} else {
		fmt.Println("PACKAGE DESCRIPTION")
		fmt.Printf("Name: %s\n", pkgList[keyword].Name)
		fmt.Printf("Version: %s\n", pkgList[keyword].Version)
		fmt.Printf("Description: %s\n", pkgList[keyword].Description)
		fmt.Printf("Dependencies: %s\n", genBuildList(pkgList[keyword]))
	}
}

func search() {
	searchFS := flag.NewFlagSet("search", flag.ExitOnError)
	searchFS.Parse(flag.Args())
	keyword := searchFS.Arg(1)
	loadPkgList()
	fmt.Printf("searching for %s...\n", keyword)
	sregex := regexp.MustCompile(`^[A-Za-z0-9]*` + keyword + `[A-Za-z0-9]*$`)
	for key, pkg := range pkgList {
		if sregex.MatchString(key) {
			fmt.Println(pkg.Name)
		}
	}
}

func installPkgs(keyword string) {
	pkg := pkgList[keyword]
	pkg.Download()
	pkg.DownloadSources()
	pkg.Create()
	pkg.Install()
}

func genBuildList(pkg Package) (depList []string) {
	for _, dep := range pkg.Requires {
		if len(pkgList[dep].Requires) > 0 {
			depList = append(depList, genBuildList(pkgList[dep])...)
		}
	}
	return append(depList, pkg.Name)
}

func genPkgList() (list map[string]Package) {
	list = make(map[string]Package)
	log.Println(pkgListUrl)
	resp, err := http.Get(pkgListUrl)
	check(err)
	if resp.StatusCode != 200 {
		log.Fatalln(resp.Status)
	}
	body, err := ioutil.ReadAll(resp.Body)
	check(err)
	packages := strings.Split(string(body), "\n\n")
	for _, pkgString := range packages {
		pkg := parsePkg(pkgString)
		list[pkg.Name] = pkg
	}
	return list
}

func parsePkg(pkgString string) (pkg Package) {
	pkgLines := strings.Split(pkgString, "\n")
	if len(pkgLines) == 10 {
		pkg.Name = pkgLines[0][17:]
		pkg.Location = pkgLines[1][21:]
		pkg.Files = strings.Split(strings.TrimSpace(pkgLines[2][18:]), " ")
		pkg.Version = pkgLines[3][20:]
		pkg.DownloadUrl = strings.Split(strings.TrimSpace(pkgLines[4][20:]), " ")
		pkg.DownloadX86_64Url = strings.Split(strings.TrimSpace(pkgLines[5][28:]), " ")
		pkg.Md5Sum = strings.Split(strings.TrimSpace(pkgLines[6][19:]), " ")
		pkg.Md5SumX86_64 = strings.Split(strings.TrimSpace(pkgLines[7][26:]), " ")
		pkg.Requires = strings.Split(strings.TrimSpace(pkgLines[8][21:]), " ")
		pkg.Description = pkgLines[9][30:]

	}
	return pkg
}

func userHomeDir() string {
	usr, err := user.Current()
	check(err)
	return usr.HomeDir
}
