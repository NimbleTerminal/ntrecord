package render

import "testing"

func TestEngineReportsLibghosttyBuild(t *testing.T) {
	e, err := EngineInfo()
	if err != nil {
		t.Fatal(err)
	}
	if e.Version == "" || !e.KittyGraphics {
		t.Fatalf("%+v: sigmamax needs a libghostty-vt with Kitty graphics", e)
	}
}
