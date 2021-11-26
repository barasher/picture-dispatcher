package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestLoadConf(t *testing.T) {
	expDateFields := []dateField{
		{"CreateDate", "2006:01:02 15:04:05"},
		{"Media Create Date", "2006:01:02 15:04:05"},
	}
	var tcs = []struct {
		tcID                string
		confFile            string
		expError            bool
		expLoggingLevel     string
		expBatchSize        int
		expDateFields       []dateField
		expOutputDateFormat string
	}{
		{"nominal", "testdata/conf/nominal.json", false, "warn", 2, expDateFields, "2016+01"},
		{"default", "testdata/conf/default.json", false, defaultLoggingLevel, 0, expDateFields, defaultOutputDateFormat},
		{"unparsable", "testdata/conf/unparsable.json", true, "", 0, nil, ""},
		{"nonExisting", "testdata/conf/nonExisting.json", true, "", 0, nil, ""},
		{"noDateField", "testdata/conf/noDateField.json", true, "", 0, nil, ""},
	}

	for _, tc := range tcs {
		t.Run(tc.tcID, func(t *testing.T) {
			c, err := loadConf(tc.confFile)
			assert.Equal(t, tc.expError, err != nil)
			if !tc.expError {
				assert.Equal(t, tc.expLoggingLevel, c.LoggingLevel)
				assert.Equal(t, tc.expBatchSize, c.ThreadCount)
				assert.Equal(t, tc.expDateFields, c.DateFields)
			}
		})
	}
}

func TestSetLoggingLevel(t *testing.T) {
	var tcs = []struct {
		tcID       string
		preLvl     zerolog.Level
		inLvl      string
		expSuccess bool
		expLvl     zerolog.Level
	}{
		{"debug", zerolog.InfoLevel, "debug", true, zerolog.DebugLevel},
		{"info", zerolog.DebugLevel, "info", true, zerolog.InfoLevel},
		{"warn", zerolog.DebugLevel, "warn", true, zerolog.WarnLevel},
		{"undefined", zerolog.DebugLevel, "undefined", false, zerolog.WarnLevel},
		{"empty", zerolog.DebugLevel, "", true, zerolog.DebugLevel},
	}

	preTestLvl := zerolog.GlobalLevel()
	defer zerolog.SetGlobalLevel(preTestLvl)

	for _, tc := range tcs {
		t.Run(tc.tcID, func(t *testing.T) {
			zerolog.SetGlobalLevel(tc.preLvl)
			err := setLoggingLevel(tc.inLvl)
			if tc.expSuccess {
				assert.Nil(t, err)
				assert.Equal(t, tc.expLvl, zerolog.GlobalLevel())
			} else {
				assert.NotNil(t, err)
			}
			assert.Equal(t, tc.expSuccess, err == nil)
		})
	}
}

func checkExist(t *testing.T, path string, shouldExist bool) {
	_, err := os.Stat(path)
	if shouldExist {
		assert.Nil(t, err)
	} else {
		assert.True(t, os.IsNotExist(err))
	}
}

func copy(from, to string) error {
	source, err := os.Open(from)
	if err != nil {
		return err
	}
	defer source.Close()
	destination, err := os.Create(to)
	if err != nil {
		return err
	}
	defer destination.Close()
	_, err = io.Copy(destination, source)
	return err
}

func TestDoMainNominal(t *testing.T) {
	tmpDir := t.TempDir()
	inDir := filepath.Join(tmpDir, "in")
	assert.Nil(t, os.Mkdir(inDir, 0777))
	jpgFile := filepath.Join(inDir, "20190404_131804.jpg")
	assert.Nil(t, copy("testdata/input/20190404_131804.jpg", jpgFile))
	movFile := filepath.Join(inDir, "20190404_131804.MOV")
	assert.Nil(t, copy("testdata/input/20190404_131804.jpg", movFile))
	outDir := filepath.Join(tmpDir, "out")
	assert.Nil(t, os.Mkdir(outDir, 0777))

	ret := doMain([]string{"osef", "-c", "testdata/conf/nominal.json", "-s", inDir, "-d", outDir})
	assert.Equal(t, retOk, ret)

	checkExist(t, jpgFile, false)
	checkExist(t, movFile, false)
	checkExist(t, filepath.Join(outDir, "2019+04", "20190404_131804.jpg"), true)

}
