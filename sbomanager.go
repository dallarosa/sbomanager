package main

import (
	"archive/tar"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path"
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
	flag.String("search", "", "package to search")
	flag.String("install", "", "package to install")
	flag.String("show", "", "show package details")
	flag.Bool("update", false, "update package list")
	flag.Parse()

	versionFile, err := ioutil.ReadFile("/etc/slackware-version")
	check(err)

	version = strings.Split(strings.Trim(string(versionFile), "\n"), " ")[1]

	pkgListUrl = baseUrl + version + "/SLACKBUILDS.TXT"

	cmd := exec.Command("uname", "-m")
	out, err := cmd.Output()
	arch = strings.Trim(string(out), "\n")

}

func main() {
	if flag.NFlag() > 1 {
		fmt.Println("too many flags set")
		return
	}
	flag.Visit(runCommand)

}

func runCommand(sFlag *flag.Flag) {
	keyword := sFlag.Value.String()
	switch sFlag.Name {
	case "search":
		loadPkgList()
		search(keyword)
	case "update":
		update()
	case "show":
		loadPkgList()
		show(keyword)
	case "install":
		loadPkgList()
		if pkgList[keyword].Name == "" {
			fmt.Println("Package not found")
		} else {
			buildList := genBuildList(pkgList[keyword])
			fmt.Println("Building the following packages:")
			fmt.Println(buildList)
			fmt.Printf("(Y/n)")
			c := ""
			fmt.Scanln(&c)
			if strings.ToLower(c) == "y" || c == "\n" {
				for _, pkgName := range buildList {
					install(pkgName)
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

func show(keyword string) {
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

func search(keyword string) {
	fmt.Printf("searching for %s...\n", keyword)
	sregex := regexp.MustCompile(`^[A-Za-z0-9]*` + keyword + `[A-Za-z0-9]*$`)
	for key, pkg := range pkgList {
		if sregex.MatchString(key) {
			fmt.Println(pkg.Name)
		}
	}
}

func install(keyword string) {
	downloadPackage(pkgList[keyword])
	downloadSources(pkgList[keyword])
	createPackage(pkgList[keyword])
}

func createPackage(pkg Package) {
	log.Println("Creating package...")
	err := os.Chdir(tmpDir + pkg.Name)
	check(err)
	cmd := exec.Command("./" + pkg.Name + ".SlackBuild")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout
	err = cmd.Run()
	check(err)
}

func downloadSources(pkg Package) {
	for _, sourceUrl := range pkg.DownloadUrl {
		log.Printf("Downloading source at %s\n", sourceUrl)

		resp, err := http.Get(sourceUrl)
		check(err)

		url, err := url.Parse(sourceUrl)
		check(err)
		_, fileName := path.Split(url.Path)
		check(err)

		out, err := os.Create(tmpDir + pkg.Name + "/" + fileName)
		check(err)
		_, err = io.Copy(out, resp.Body)
		check(err)

		err = out.Close()
		check(err)
		err = resp.Body.Close()
		check(err)
	}
}

func downloadPackage(pkg Package) {
	err := os.MkdirAll(tmpDir, 0755)
	check(err)

	pkgUrl := baseUrl + version + pkg.Location[1:] + ".tar.gz"
	log.Printf("Downloading SBO package at %s\n", pkgUrl)

	resp, err := http.Get(pkgUrl)
	defer resp.Body.Close()
	check(err)

	log.Printf("Extracting package...\n")
	tr := tar.NewReader(resp.Body)
	slackbuildFile := pkg.Name + "/" + pkg.Name + ".SlackBuild"
	if tr != nil {
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatalln(err)
			}
			switch hdr.Typeflag {
			case tar.TypeDir:
				dirPath := tmpDir + hdr.Name
				log.Println(dirPath)
				err = os.Mkdir(tmpDir+hdr.Name, os.ModeDir|0755)
			default:
				filePath := tmpDir + hdr.Name
				log.Println(filePath)
				out, err := os.Create(tmpDir + hdr.Name)
				check(err)
				_, err = io.Copy(out, tr)
				check(err)
				err = out.Close()
				check(err)
				if hdr.Name == slackbuildFile {
					err = os.Chmod(tmpDir+slackbuildFile, 0755)
					check(err)
				}
			}
		}
	}
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

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func check(err error) {
	if err != nil {
		log.Fatalln(err)
		return
	}
}
