package ninite

import (
	"reflect"
	"testing"
)

func TestParseNinite(t *testing.T) {
	got, err := Parse([]string{"7zip steam", "https://ninite.com/discord-7zip-.net8/ninite.exe", "paint.net,operaChromium"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"7zip", "steam", "discord", ".net8", "paint.net", "operaChromium"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if Page(want[:2]) != "https://ninite.com/7zip-steam/" || Installer(want[:2]) != "https://ninite.com/7zip-steam/ninite.exe" {
		t.Error("wrong ninite links")
	}
	if Page(nil) != Home {
		t.Error("an empty selection is the home page")
	}
	for _, bad := range []string{"https://example.com/7zip/", "steam;calc", "a/b"} {
		if _, err := Parse([]string{bad}); err == nil {
			t.Errorf("%q should be refused", bad)
		}
	}
	if u := Unknown([]string{"steam", "nope"}); !reflect.DeepEqual(u, []string{"nope"}) {
		t.Errorf("unknown = %v", u)
	}
}
