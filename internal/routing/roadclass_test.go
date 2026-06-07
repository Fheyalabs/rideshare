package routing

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		highway   string
		wantClass RoadClass
		wantDrive bool
	}{
		{"motorway", ClassMotorway, true},
		{"trunk", ClassTrunk, true},
		{"primary", ClassPrimary, true},
		{"residential", ClassLocal, true},
		{"footway", ClassNone, false},
		{"", ClassNone, false},
	}
	for _, c := range cases {
		got, drive := Classify(c.highway)
		if got != c.wantClass || drive != c.wantDrive {
			t.Errorf("Classify(%q)=%v,%v want %v,%v", c.highway, got, drive, c.wantClass, c.wantDrive)
		}
	}
	if DefaultSpeedKmh(ClassMotorway) <= DefaultSpeedKmh(ClassLocal) {
		t.Error("motorway default speed must exceed local")
	}
}
