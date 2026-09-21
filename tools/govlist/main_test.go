package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const fixture = `<?xml version="1.0" encoding="UTF-8"?>
<p:overheidsorganisaties xmlns:p="https://organisaties.overheid.nl/static/schema/oo/export/2.6.9">
<p:organisaties>
  <p:organisatie>
    <p:naam>Gemeente Diemen</p:naam>
    <p:types><p:type>Gemeente</p:type></p:types>
    <p:contact><p:internetadressen>
      <p:internetadres><p:url>https://www.diemen.nl/contact</p:url><p:label>contact</p:label></p:internetadres>
      <p:internetadres><p:url>https://www.diemen.nl</p:url><p:label>algemeen</p:label></p:internetadres>
    </p:internetadressen></p:contact>
  </p:organisatie>
  <p:organisatie>
    <p:naam>Gemeente Opgeheven</p:naam>
    <p:types><p:type>Gemeente</p:type></p:types>
    <p:eindDatum>2019-01-01</p:eindDatum>
    <p:contact><p:internetadressen><p:internetadres><p:url>https://www.opgeheven.nl</p:url><p:label>algemeen</p:label></p:internetadres></p:internetadressen></p:contact>
  </p:organisatie>
  <p:organisatie>
    <p:naam>Gemeente Fuseert Later</p:naam>
    <p:types><p:type>Gemeente</p:type></p:types>
    <p:eindDatum>2027-01-01</p:eindDatum>
    <p:contact><p:internetadressen><p:internetadres><p:url>https://fuseertlater.nl/</p:url><p:label>algemeen</p:label></p:internetadres></p:internetadressen></p:contact>
  </p:organisatie>
  <p:organisatie>
    <p:naam>Ministerie van Algemene Zaken</p:naam>
    <p:types><p:type>Ministerie</p:type></p:types>
    <p:contact><p:internetadressen><p:internetadres><p:url>www.rijksoverheid.nl</p:url></p:internetadres></p:internetadressen></p:contact>
    <p:organisaties>
      <p:organisatie>
        <p:naam>Subonderdeel</p:naam>
        <p:types><p:type>Agentschap</p:type></p:types>
        <p:contact><p:internetadressen><p:internetadres><p:url>https://sub.example.nl</p:url><p:label>algemeen</p:label></p:internetadres></p:internetadressen></p:contact>
      </p:organisatie>
    </p:organisaties>
  </p:organisatie>
  <p:organisatie>
    <p:naam>Ministerie van Financiën</p:naam>
    <p:types><p:type>Ministerie</p:type></p:types>
    <p:contact><p:internetadressen><p:internetadres><p:url>https://www.rijksoverheid.nl/ministeries/fin</p:url><p:label>algemeen</p:label></p:internetadres></p:internetadressen></p:contact>
  </p:organisatie>
  <p:organisatie>
    <p:naam>Stichting Iets</p:naam>
    <p:types><p:type>Overheidsstichting of -vereniging</p:type></p:types>
    <p:contact><p:internetadressen><p:internetadres><p:url>https://iets.nl</p:url><p:label>algemeen</p:label></p:internetadres></p:internetadressen></p:contact>
  </p:organisatie>
</p:organisaties>
</p:overheidsorganisaties>`

func TestParse(t *testing.T) {
	got, err := parse(strings.NewReader(fixture), "2026-09-21")
	if err != nil {
		t.Fatal(err)
	}
	want := []entry{
		{domain: "diemen.nl", name: "Gemeente Diemen"},
		{domain: "fuseertlater.nl", name: "Gemeente Fuseert Later"},
		{domain: "rijksoverheid.nl", name: "Ministerie van Algemene Zaken"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestRunWritesCSV(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "exportOO.xml")
	out := filepath.Join(dir, "government.csv")
	os.WriteFile(in, []byte(fixture), 0o644)
	if err := run(in, out, "2026-09-21"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(out)
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if lines[0] != "domain,name,source" || len(lines) != 4 ||
		lines[1] != "diemen.nl,Gemeente Diemen,https://organisaties.overheid.nl/archive/exportOO.xml" {
		t.Fatalf("unexpected CSV:\n%s", b)
	}
}
