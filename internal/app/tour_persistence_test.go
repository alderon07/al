package app

import "testing"

func TestTourSchedulePreservesSeenState(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	service := DefaultServices()
	if err := service.scheduleTour(); err != nil {
		t.Fatal(err)
	}
	if !service.tourShouldShow() {
		t.Fatal("scheduled tour was not visible")
	}
	if err := service.markTourSeen(); err != nil {
		t.Fatal(err)
	}
	if err := service.scheduleTour(); err != nil {
		t.Fatal(err)
	}
	if service.tourShouldShow() {
		t.Fatal("completed tour was rescheduled")
	}
}
