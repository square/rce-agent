package cmd_test

import (
	"testing"

	"github.com/square/rce-agent/cmd"
)

func TestOneId(t *testing.T) {
	repo := cmd.NewRepo()

	command := cmd.Cmd{Id: "some_id"}
	repo.Add(&command)
	all_ids := repo.All()

	if len(all_ids) != 1 {
		t.Errorf("Expected 1 ID, got %d", len(all_ids))
	}

	if all_ids[0] != "some_id" {
		t.Errorf("Expected the ID to be 'some_id', but got %s", all_ids[0])
	}
}

func TestMultipleIds(t *testing.T) {
	repo := cmd.NewRepo()

	expectedIds := []string{"first ID", "second ID", "third ID", "fourth ID"}

	for _, id := range expectedIds {
		command := cmd.Cmd{Id: id}
		repo.Add(&command)
	}

	allIds := repo.All()

	if len(allIds) != len(expectedIds) {
		t.Errorf("Expected %d IDs, got %d", len(expectedIds), len(allIds))
	}

	for _, foundId := range allIds {
		seen := false
		for _, expectedId := range expectedIds {
			if expectedId == foundId {
				seen = true
			}
		}
		if !seen {
			t.Errorf("Found unexpected ID: '%s'", foundId)
		}
	}

	for _, expectedId := range expectedIds {
		seen := false
		for _, foundId := range allIds {
			if expectedId == foundId {
				seen = true
			}
		}
		if !seen {
			t.Errorf("Expected ID not found: '%s'", expectedId)
		}
	}

}
