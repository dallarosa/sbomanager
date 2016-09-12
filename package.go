package main

import (
	"archive/tar"
	"bytes"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
)

type Package struct {
	Name              string
	Location          string
	Files             []string
	Version           string
	DownloadUrl       []string
	DownloadX86_64Url []string
	Md5Sum            []string
	Md5SumX86_64      []string
	Requires          []string
	Description       string
	PackageFilePath   string
}

func (p *Package) Create() {
	log.Println("Creating package...")
	err := os.Chdir(tmpDir + p.Name)
	check(err, getLine())
	cmd := exec.Command("/bin/sh", p.Name+".SlackBuild")
	output, err := cmd.CombinedOutput()
	splitOutput := bytes.Split(output, []byte{0x0a})
	check(err, getLine())

	pkgName := bytes.Split(splitOutput[len(splitOutput)-3], []byte{0x20})[2]

	p.PackageFilePath = string(pkgName)

}

func (p *Package) IsInstalled() bool {
	exp := "/var/log/packages/" + p.Name + "-*"
	files, err := filepath.Glob(exp)

	if err != nil {
		log.Println("failed")
		log.Println(err)
		return false
	}

	if len(files) == 0 {
		return false
	}

	return true
}

func (p *Package) Install() {
	if p.PackageFilePath != "" {
		cmd := exec.Command("/sbin/installpkg", p.PackageFilePath)
		cmd.Stderr = os.Stdout
		cmd.Stdout = os.Stdout
		err := cmd.Run()
		check(err)
	} else {
		log.Fatalln("Couldn't install package")
	}
}

func (p *Package) DownloadSources() {
	for _, sourceUrl := range p.DownloadUrl {
		log.Printf("Downloading source at %s\n", sourceUrl)

		resp, err := http.Get(sourceUrl)
		check(err)

		url, err := url.Parse(sourceUrl)
		check(err)
		_, fileName := path.Split(url.Path)
		check(err)

		out, err := os.Create(tmpDir + p.Name + "/" + fileName)
		check(err)
		_, err = io.Copy(out, resp.Body)
		check(err)

		err = out.Close()
		check(err)
		err = resp.Body.Close()
		check(err)
	}
}

func (p *Package) Download() {
	err := os.MkdirAll(tmpDir, 0755)
	check(err)

	pkgUrl := baseUrl + version + p.Location[1:] + ".tar.gz"
	log.Printf("Downloading SBO package at %s\n", pkgUrl)

	resp, err := http.Get(pkgUrl)
	defer resp.Body.Close()
	check(err)

	log.Printf("Extracting package...\n")
	tr := tar.NewReader(resp.Body)
	slackbuildFile := p.Name + "/" + p.Name + ".SlackBuild"
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
