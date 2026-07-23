package services

import (
	"bytes"
	"embed"
	"encoding/base64"
	"html/template"
)

//go:embed templates/*.html templates/logo.png
var templateFS embed.FS

// logoDataURI is the header glyph inlined as a base64 data URI, so the
// email never depends on an externally hosted image being reachable (or
// on images being enabled at all in the recipient's client). Typed as
// template.URL — a value html/template trusts as pre-vetted — because a
// plain string data: URI gets stripped to "#ZgotmplZ" by the default
// escaper, which only allows http(s)/mailto for untrusted string values.
// This is safe here since the bytes come from our own embedded asset,
// never from user input.
var logoDataURI = mustLogoDataURI()

func mustLogoDataURI() template.URL {
	data, err := templateFS.ReadFile("templates/logo.png")
	if err != nil {
		panic(err)
	}
	return template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(data))
}

var templateFuncs = template.FuncMap{
	"logoDataURI": func() template.URL { return logoDataURI },
}

// mustParse builds an independent template set from the shared layout
// plus one content file, so each email's "content" definition can't
// collide with another's when parsed.
func mustParse(contentFile string) *template.Template {
	return template.Must(template.New("layout").Funcs(templateFuncs).ParseFS(
		templateFS, "templates/layout.html", "templates/"+contentFile,
	))
}

var confirmTmpl = mustParse("confirm.html")
var broadcastTmpl = mustParse("broadcast.html")

// ConfirmEmailData is the template data for the subscription-confirmation
// email.
type ConfirmEmailData struct {
	Link string
}

// BroadcastEmailData is the template data for the new-post broadcast
// email.
type BroadcastEmailData struct {
	PostTitle       string
	PostURL         string
	UnsubscribeLink string
}

func render(tmpl *template.Template, data any) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "layout", data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// RenderConfirm renders the subscription-confirmation email body.
func RenderConfirm(data ConfirmEmailData) (string, error) {
	return render(confirmTmpl, data)
}

// RenderBroadcast renders the new-post broadcast email body.
func RenderBroadcast(data BroadcastEmailData) (string, error) {
	return render(broadcastTmpl, data)
}
