package internal

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/barasher/go-exiftool"
	"github.com/rs/zerolog"
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

func TestGuessDateNominal(t *testing.T) {
	fields := map[string]interface{}{
		"a":          "b",
		"CreateDate": "2018:01:02 03:04:05",
	}
	fm := exiftool.FileMetadata{File: "a", Fields: fields}
	c := buildDefaultClassifier(t, 2)
	got, err := c.guessDate(fm)
	assert.Nil(t, err)
	assert.Equal(t, 2018, got.Year())
	assert.Equal(t, time.January, got.Month())
	assert.Equal(t, 2, got.Day())
	assert.Equal(t, 3, got.Hour())
	assert.Equal(t, 4, got.Minute())
	assert.Equal(t, 5, got.Second())
}

func TestGuessDateWithoutDateField(t *testing.T) {
	fields := map[string]interface{}{
		"a": "b",
	}
	fm := exiftool.FileMetadata{File: "a", Fields: fields}
	c := buildDefaultClassifier(t, 2)
	_, err := c.guessDate(fm)
	assert.Equal(t, errNoDateFound, err)
}

func TestGuessDateUnparsableDate(t *testing.T) {
	fields := map[string]interface{}{
		"a":          "b",
		"CreateDate": "unparsableDate",
	}
	fm := exiftool.FileMetadata{File: "a", Fields: fields}
	c := buildDefaultClassifier(t, 2)
	_, err := c.guessDate(fm)
	assert.NotNil(t, err)
	assert.NotEqual(t, errNoDateFound, err)
}

func checkExist(t *testing.T, path string, shouldExist bool) {
	_, err := os.Stat(path)
	if shouldExist {
		assert.Nil(t, err)
	} else {
		assert.True(t, os.IsNotExist(err))
	}
}

func TestMoveFiles(t *testing.T) {
	tmpDir := t.TempDir()
	inDir := filepath.Join(tmpDir, "in")
	os.MkdirAll(inDir, 0777)
	outDir := filepath.Join(tmpDir, "out")
	os.MkdirAll(outDir, 0777)
	inFile := filepath.Join(inDir, "20190404_131804.jpg")
	assert.Nil(t, copy("../testdata/input/20190404_131804.jpg", inFile))

	ctx, cancel := context.WithCancel(context.TODO())
	moveChan := make(chan moveAction, 2)
	moveChan <- moveAction{from: inFile, to: "2019_04"}
	close(moveChan)

	c := buildDefaultClassifier(t, 2)
	c.moveFiles(ctx, cancel, outDir, moveChan)

	checkExist(t, "../testdata/tmp/batch/TestMoveFilesNominal/in/20190404_131804.jpg", false)
	checkExist(t, "../testdata/tmp/batch/TestMoveFilesNominal/out/2019_04/20190404_131804.jpg", true)
}

func TestClassify(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	tmpDir := t.TempDir() + "TestClassify"
	inDir := filepath.Join(tmpDir, "in")
	os.MkdirAll(inDir, 0777)
	subDir := filepath.Join(inDir, "subFolder")
	os.MkdirAll(subDir, 0777)
	assert.Nil(t, copy("../testdata/input/20190404_131804.jpg", filepath.Join(subDir, "20190404_131805.jpg")))
	assert.Nil(t, copy("../testdata/input/20190404_131804.jpg", filepath.Join(subDir, "20190404_131806.jpg")))
	assert.Nil(t, copy("../testdata/input/subFolder/noDate.txt", filepath.Join(subDir, "noDate.txt")))
	assert.Nil(t, copy("../testdata/input/20190404_131804.jpg", filepath.Join(inDir, "20190404_131804.jpg")))

	outDir := filepath.Join(tmpDir, "out")
	os.MkdirAll(outDir, 0777)
	c := buildDefaultClassifier(t, 2)
	c.Classify(inDir, outDir)

	checkExist(t, filepath.Join(subDir, "noDate.txt"), true)
	checkExist(t, filepath.Join(subDir, "20190404_131805.jpg"), false)
	checkExist(t, filepath.Join(subDir, "20190404_131806.jpg"), false)
	checkExist(t, filepath.Join(inDir, "20190404_131804.jpg"), false)
	checkExist(t, filepath.Join(outDir, "2019_04", "20190404_131804.jpg"), true)
	checkExist(t, filepath.Join(outDir, "2019_04", "20190404_131805.jpg"), true)
	checkExist(t, filepath.Join(outDir, "2019_04", "20190404_131806.jpg"), true)
}
