package app

type CatalogInitOptions struct {
	Source            string
	Shell             string
	CatalogPath       string
	Apply             bool
	ReviewedSourceSHA string
	ReviewedHEAD      string
	ReviewedBlob      string
	StartupPaths      []string
}
