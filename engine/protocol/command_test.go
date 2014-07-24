package protocol

import "testing"

func TestAboutValidatePass(t *testing.T) {
	// about requires state > 0 to pass validation
	state := State{State: 1}
	about := About{}
	if pass, err := about.Validate(&state, nil); !pass {
		t.Fatal("About.Validate: ", err)
	}
}

func TestAboutValidateFail(t *testing.T) {
	// about requires state > 0 to pass validation
	state := State{State: 0}
	about := About{}
	if pass, err := about.Validate(&state, nil); pass {
		t.Fatal("About.Validate: should fail with stat < 1")
	} else {
		if err.Error() != "The about command requires a state of non-zero" {
			t.Fatal("About.Validate: error message is not as expected")
		}
	}
}

func TestAboutRunFail(t *testing.T) {
	state := State{State: 0}
	about := About{}

	if reply, err := about.Run(&state, nil); err == nil {
		t.Fatal("About.Run: should fail with state < 1")
	} else {
		if reply.Params["message"] != err.Error() {
			t.Fatal("About.Run: error in reply does not match programmatic error")
		}
	}
}

func TestAboutRunPass(t *testing.T) {
	state := State{State: 1}
	about := About{}

	if reply, err := about.Run(&state, nil); err != nil {
		t.Fatal("About.Run: should pass validate with state of 1")
	} else {
		if reply.Params["reply"] != "OK" {
			t.Fatal("About.Run: reply does not indicate command was successful")
		}
		if reply.Params["text"] != "Go Doctor about text" {
			t.Fatal("About.Run: reply has incorrect about text")
		}
	}
}
