package main

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
}

func (p *Package) pkgFileName() string {
	return ""
}
