package internal

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func buildDefaultClassifier(t *testing.T, threadCount int) *Classifier {
	c, err := NewClassifier(
		OptThreadCount(threadCount),
		OptDateFields(map[string]string{"CreateDate": "2006:01:02 15:04:05"}),
	)
	assert.Nil(t, err)
	return c
}

func TestListFiles(t *testing.T) {
	var tcs = []struct {
		tcID        string
		folder      string
		expFiles    []string
		expCanceled bool
	}{
		{
			tcID:        "nominal",
			folder:      "../testdata/input/",
			expFiles:    []string{"../testdata/input/20190404_131804.jpg", "../testdata/input/subFolder/20190404_131805.jpg"},
			expCanceled: false,
		},
		{
			tcID:        "nonExistingFolder",
			folder:      "../nonExistingFolder/",
			expFiles:    []string{},
			expCanceled: true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.tcID, func(t *testing.T) {

			ctx, cancel := context.WithCancel(context.TODO())
			filesChan := make(chan string, 10)

			c := buildDefaultClassifier(t, 1)
			c.listFiles(ctx, cancel, tc.folder, filesChan)

			files := make([]string, 10)
			for f := range filesChan {
				files = append(files, f)
			}

			assert.Subset(t, files, tc.expFiles)
			select {
			case <-ctx.Done():
				assert.True(t, tc.expCanceled)
			default:
				assert.False(t, tc.expCanceled)
			}

		})
	}
}

func TestGetMoveActions(t *testing.T) {
	var tcs = []struct {
		tcID       string
		files      []string
		expActions []moveAction
	}{
		{
			tcID: "nominal",
			files: []string{
				"../testdata/input/20190404_131804.jpg",
				"../testdata/input/subFolder/20190404_131805.jpg",
				"../testdata/input/subFolder/20190404_131806.jpg",
			},
			expActions: []moveAction{
				{from: "../testdata/input/20190404_131804.jpg", to: "2019_04"},
				{from: "../testdata/input/subFolder/20190404_131805.jpg", to: "2019_04"},
				{from: "../testdata/input/subFolder/20190404_131806.jpg", to: "2019_04"},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.tcID, func(t *testing.T) {
			func() {
				ctx, cancel := context.WithCancel(context.TODO())
				fileChan := make(chan string, 10)
				actionChan := make(chan moveAction, 10)

				for _, s := range tc.files {
					fileChan <- s
				}
				close(fileChan)

				c := buildDefaultClassifier(t, 2)
				c.getMoveActions(ctx, cancel, fileChan, actionChan)

				actions := []moveAction{}
				for ma := range actionChan {
					actions = append(actions, ma)
				}
				assert.Subset(t, actions, tc.expActions)
			}()

		})
	}
}
