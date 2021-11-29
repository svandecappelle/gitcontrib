package main

import "testing"

var existingUser = "Steeve Vandecappelle"
var unknownUser = "Mybest Friend"


func TestFolderIsRepo(t *testing.T) {
	t.Run("Check folder is a repository", func(t *testing.T) {
		if !isRepo(".") {
			t.Errorf("isRepo should be true")
		}
	})
}

func TestStatRepoFromFolder(t *testing.T) {
	currentRepo := []string{"."}

	t.Run("try to test for all users", func (t *testing.T) {
		err := Stats(nil, 4, currentRepo, "")
		if err != nil {
			t.Errorf("Error testing stat from repository folder %s", err)
		}
	})


	t.Run("try to test for one user", func (t *testing.T) {
		err := Stats(&unknownUser, 4, currentRepo, "")
		if err != nil {
			t.Errorf("Error testing stat from repository folder %s", err)
		}
	})
	
	t.Run("try to test for one user", func (t *testing.T) {
		err := Stats(&existingUser, 4, currentRepo, "")
		if err != nil {
			t.Errorf("Error testing stat from repository folder %s", err)
		}
	})
}
