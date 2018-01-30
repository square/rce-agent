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

	command := cmd.Cmd{Id: "first ID"}
	repo.Add(&command)
	command = cmd.Cmd{Id: "second ID"}
	repo.Add(&command)
	all_ids := repo.All()

	if len(all_ids) != 2 {
		t.Errorf("Expected 2 IDs, got %d", len(all_ids))
	}

	expected := []string{"first ID", "second ID"}
	for i := range all_ids {
		if all_ids[i] != expected[i] {
			t.Errorf("Expected ID %d to be '%s', but was '%s'", i, expected[i], all_ids[i])
		}
	}
}
