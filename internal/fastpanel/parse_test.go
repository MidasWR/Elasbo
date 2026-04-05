package fastpanel

import "testing"

func TestParseSitesList_header(t *testing.T) {
	const sample = `ID      SERVER_NAME             ALIASES                         OWNER                   MODE    PHP_VERSION     IPS             DOCUMENT_ROOT
1       example.com             www.example.com                 example_com_usr         mpm_itk 82              127.0.0.1   /var/www/example_com_usr/data/www/example.com
2       other.test                                                           usr2         php_fpm 84              127.0.0.1   /var/www/usr2/data/www/other.test
`
	sites, err := ParseSitesList(sample)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 2 {
		t.Fatalf("got %d sites", len(sites))
	}
	if sites[0].ID != 1 || sites[0].ServerName != "example.com" {
		t.Fatalf("site0: %+v", sites[0])
	}
	if sites[0].DocumentRoot != "/var/www/example_com_usr/data/www/example.com" {
		t.Fatalf("root0: %s", sites[0].DocumentRoot)
	}
	if sites[1].ServerName != "other.test" {
		t.Fatalf("site1: %+v", sites[1])
	}
}

func TestParseSitesList_heuristic(t *testing.T) {
	line := "99  legacy.org  www.legacy.org  owner_x  fcgi  8.1  127.0.0.1  /var/www/x/data/www/legacy.org"
	sites, err := ParseSitesList(line + "\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 1 || sites[0].ID != 99 || sites[0].ServerName != "legacy.org" {
		t.Fatalf("%+v", sites)
	}
}
