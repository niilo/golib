// Copyright (c) 2013 - Cloud Instruments Co., Ltd.
//
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are met:
//
// 1. Redistributions of source code must retain the above copyright notice, this
//    list of conditions and the following disclaimer.
// 2. Redistributions in binary form must reproduce the above copyright notice,
//    this list of conditions and the following disclaimer in the documentation
//    and/or other materials provided with the distribution.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
// ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
// WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
// DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT OWNER OR CONTRIBUTORS BE LIABLE FOR
// ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
// (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
// LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND
// ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
// SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package io

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Common constants
const (
	rollingLogHistoryDelimiter = "."
)

// Types of the rolling writer: roll by date, by time, etc.
type RollingType uint8

const (
	RollingTypeSize = iota
	RollingTypeTime
)

type RollingIntervalType uint8

const (
	RollingIntervalAny = iota
	RollingIntervalDaily
)

// File and directory permitions.
const (
	defaultFilePermissions      = 0666
	defaultDirectoryPermissions = 0767
)

var rollingInvervalTypesStringRepresentation = map[RollingIntervalType]string{
	RollingIntervalDaily: "daily",
}

func RollingIntervalTypeFromString(RollingTypeStr string) (RollingIntervalType, bool) {
	for tp, tpStr := range rollingInvervalTypesStringRepresentation {
		if tpStr == RollingTypeStr {
			return tp, true
		}
	}

	return 0, false
}

var RollingTypesStringRepresentation = map[RollingType]string{
	RollingTypeSize: "size",
	RollingTypeTime: "date",
}

func RollingTypeFromString(RollingTypeStr string) (RollingType, bool) {
	for tp, tpStr := range RollingTypesStringRepresentation {
		if tpStr == RollingTypeStr {
			return tp, true
		}
	}

	return 0, false
}

// Old logs archivation type.
type RollingArchiveType uint8

const (
	RollingArchiveNone = iota
	RollingArchiveZip
)

var RollingArchiveTypesStringRepresentation = map[RollingArchiveType]string{
	RollingArchiveNone: "none",
	RollingArchiveZip:  "zip",
}

func RollingArchiveTypeFromString(RollingArchiveTypeStr string) (RollingArchiveType, bool) {
	for tp, tpStr := range RollingArchiveTypesStringRepresentation {
		if tpStr == RollingArchiveTypeStr {
			return tp, true
		}
	}

	return 0, false
}

// Default names for different archivation types
var RollingArchiveTypesDefaultNames = map[RollingArchiveType]string{
	RollingArchiveZip: "log.zip",
}

// RollerVirtual is an interface that represents all virtual funcs that are
// called in different rolling writer subtypes.
type RollerVirtual interface {
	needsToRoll() (bool, error)                     // Returns true if needs to switch to another file.
	isFileTailValid(tail string) bool               // Returns true if logger roll file tail (part after filename) is ok.
	sortFileTailsAsc(fs []string) ([]string, error) // Sorts logger roll file tails in ascending order of their creation by logger.

	// Creates a new froll history file using the contents of current file and filename of the latest roll.
	// If lastRollFileTail is empty (""), then it means that there is no latest roll (current is the first one)
	getNewHistoryFileNameTail(lastRollFileTail string) string
	getCurrentModifiedFileName(OriginalFileName string) string // Returns filename modified according to specific logger rules
}

// RollingFileWriter writes received messages to a file, until time Interval passes
// or file exceeds a specified limit. After that the current log file is renamed
// and writer starts to log into a new file. You can set a limit for such renamed
// files count, if you want, and then the rolling writer would delete older ones when
// the files count exceed the specified limit.
type RollingFileWriter struct {
	FileName         string // current file name. May differ from original in date rolling loggers
	OriginalFileName string // original one
	CurrentDirPath   string
	CurrentFile      *os.File
	CurrentFileSize  int64
	RollingType      RollingType // Rolling mode (Files roll by size/date/...)
	ArchiveType      RollingArchiveType
	ArchivePath      string
	MaxRolls         int
	Self             RollerVirtual // Used for virtual calls
}

func NewRollingFileWriter(fpath string, rtype RollingType, atype RollingArchiveType, apath string, maxr int) (*RollingFileWriter, error) {
	rw := new(RollingFileWriter)
	rw.CurrentDirPath, rw.FileName = filepath.Split(fpath)
	if len(rw.CurrentDirPath) == 0 {
		rw.CurrentDirPath = "."
	}
	rw.OriginalFileName = rw.FileName

	rw.RollingType = rtype
	rw.ArchiveType = atype
	rw.ArchivePath = apath
	rw.MaxRolls = maxr
	return rw, nil
}

func (rw *RollingFileWriter) getSortedLogHistory() ([]string, error) {
	files, err := getDirFilePaths(rw.CurrentDirPath, nil, true)
	if err != nil {
		return nil, err
	}
	pref := rw.OriginalFileName + rollingLogHistoryDelimiter
	var validFileTails []string
	for _, file := range files {
		if file != rw.FileName && strings.HasPrefix(file, pref) {
			tail := rw.getFileTail(file)
			if rw.Self.isFileTailValid(tail) {
				validFileTails = append(validFileTails, tail)
			}
		}
	}
	sortedTails, err := rw.Self.sortFileTailsAsc(validFileTails)
	if err != nil {
		return nil, err
	}
	validSortedFiles := make([]string, len(sortedTails))
	for i, v := range sortedTails {
		validSortedFiles[i] = rw.OriginalFileName + rollingLogHistoryDelimiter + v
	}
	return validSortedFiles, nil
}

func (rw *RollingFileWriter) createFileAndFolderIfNeeded() error {
	var err error

	if len(rw.CurrentDirPath) != 0 {
		err = os.MkdirAll(rw.CurrentDirPath, defaultDirectoryPermissions)

		if err != nil {
			return err
		}
	}

	rw.FileName = rw.Self.getCurrentModifiedFileName(rw.OriginalFileName)
	filePath := filepath.Join(rw.CurrentDirPath, rw.FileName)

	// If exists
	stat, err := os.Lstat(filePath)
	if err == nil {
		rw.CurrentFile, err = os.OpenFile(filePath, os.O_WRONLY|os.O_APPEND, defaultFilePermissions)

		stat, err = os.Lstat(filePath)
		if err != nil {
			return err
		}

		rw.CurrentFileSize = stat.Size()
	} else {
		rw.CurrentFile, err = os.Create(filePath)
		rw.CurrentFileSize = 0
	}
	if err != nil {
		return err
	}

	return nil
}

func (rw *RollingFileWriter) deleteOldRolls(history []string) error {
	if rw.MaxRolls <= 0 {
		return nil
	}

	rollsToDelete := len(history) - rw.MaxRolls
	if rollsToDelete <= 0 {
		return nil
	}

	// In all cases (archive files or not) the files should be deleted.
	for i := 0; i < rollsToDelete; i++ {
		rollPath := filepath.Join(rw.CurrentDirPath, history[i])
		err := tryRemoveFile(rollPath)
		if err != nil {
			return err
		}
	}

	return nil
}

// tryRemoveFile gives a try removing the file
// only ignoring an error when the file does not exist.
func tryRemoveFile(filePath string) (err error) {
	err = os.Remove(filePath)
	if os.IsNotExist(err) {
		err = nil
		return
	}
	return
}

func (rw *RollingFileWriter) getFileTail(FileName string) string {
	return FileName[len(rw.OriginalFileName+rollingLogHistoryDelimiter):]
}

func (rw *RollingFileWriter) Write(bytes []byte) (n int, err error) {
	if rw.CurrentFile == nil {
		err := rw.createFileAndFolderIfNeeded()
		if err != nil {
			return 0, err
		}
	}
	// needs to roll if:
	//   * file roller max file size exceeded OR
	//   * time roller Interval passed
	nr, err := rw.Self.needsToRoll()
	if err != nil {
		return 0, err
	}
	if nr {
		// First, close current file.
		err = rw.CurrentFile.Close()
		if err != nil {
			return 0, err
		}

		// Current history of all previous log files.
		// For file roller it may be like this:
		//     * ...
		//     * file.log.4
		//     * file.log.5
		//     * file.log.6
		//
		// For date roller it may look like this:
		//     * ...
		//     * file.log.11.Aug.13
		//     * file.log.15.Aug.13
		//     * file.log.16.Aug.13
		// Sorted log history does NOT include current file.
		history, err := rw.getSortedLogHistory()
		if err != nil {
			return 0, err
		}

		// Renames current file to create a new roll history entry
		// For file roller it may be like this:
		//     * ...
		//     * file.log.4
		//     * file.log.5
		//     * file.log.6
		//     n file.log.7  <---- RENAMED (from file.log)
		// Time rollers that doesn't modify file names (e.g. 'date' roller) skip this logic.
		var newHistoryName string
		var newTail string
		if len(history) > 0 {
			// Create new tail name using last history file name
			newTail = rw.Self.getNewHistoryFileNameTail(rw.getFileTail(history[len(history)-1]))
		} else {
			// Create first tail name
			newTail = rw.Self.getNewHistoryFileNameTail("")
		}

		if len(newTail) != 0 {
			newHistoryName = rw.FileName + rollingLogHistoryDelimiter + newTail
		} else {
			newHistoryName = rw.FileName
		}

		if newHistoryName != rw.FileName {
			err = os.Rename(filepath.Join(rw.CurrentDirPath, rw.FileName), filepath.Join(rw.CurrentDirPath, newHistoryName))
			if err != nil {
				return 0, err
			}
		}

		// Finally, add the newly added history file to the history archive
		// and, if after that the archive exceeds the allowed max limit, older rolls
		// must the removed/archived.
		history = append(history, newHistoryName)
		if len(history) > rw.MaxRolls {
			err = rw.deleteOldRolls(history)
			if err != nil {
				return 0, err
			}
		}

		err = rw.createFileAndFolderIfNeeded()
		if err != nil {
			return 0, err
		}
	}

	rw.CurrentFileSize += int64(len(bytes))
	return rw.CurrentFile.Write(bytes)
}

func (rw *RollingFileWriter) Close() error {
	if rw.CurrentFile != nil {
		e := rw.CurrentFile.Close()
		if e != nil {
			return e
		}
		rw.CurrentFile = nil
	}
	return nil
}

// =============================================================================================
//      Different types of rolling writers
// =============================================================================================

// --------------------------------------------------
//      Rolling writer by SIZE
// --------------------------------------------------

// RollingFileWriterSize performs roll when file exceeds a specified limit.
type RollingFileWriterSize struct {
	*RollingFileWriter
	MaxFileSize int64
}

func NewRollingFileWriterSize(fpath string, atype RollingArchiveType, apath string, maxSize int64, MaxRolls int) (*RollingFileWriterSize, error) {
	rw, err := NewRollingFileWriter(fpath, RollingTypeSize, atype, apath, MaxRolls)
	if err != nil {
		return nil, err
	}
	rws := &RollingFileWriterSize{rw, maxSize}
	rws.Self = rws
	return rws, nil
}

func (rws *RollingFileWriterSize) needsToRoll() (bool, error) {
	return rws.CurrentFileSize >= rws.MaxFileSize, nil
}

func (rws *RollingFileWriterSize) isFileTailValid(tail string) bool {
	if len(tail) == 0 {
		return false
	}
	_, err := strconv.Atoi(tail)
	return err == nil
}

type rollSizeFileTailsSlice []string

func (p rollSizeFileTailsSlice) Len() int { return len(p) }
func (p rollSizeFileTailsSlice) Less(i, j int) bool {
	v1, _ := strconv.Atoi(p[i])
	v2, _ := strconv.Atoi(p[j])
	return v1 < v2
}
func (p rollSizeFileTailsSlice) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

func (rws *RollingFileWriterSize) sortFileTailsAsc(fs []string) ([]string, error) {
	ss := rollSizeFileTailsSlice(fs)
	sort.Sort(ss)
	return ss, nil
}

func (rws *RollingFileWriterSize) getNewHistoryFileNameTail(lastRollFileTail string) string {
	v := 0
	if len(lastRollFileTail) != 0 {
		v, _ = strconv.Atoi(lastRollFileTail)
	}
	return fmt.Sprintf("%d", v+1)
}

func (rws *RollingFileWriterSize) getCurrentModifiedFileName(OriginalFileName string) string {
	return OriginalFileName
}

func (rws *RollingFileWriterSize) String() string {
	return fmt.Sprintf("Rolling file writer (By SIZE): filename: %s, archive: %s, archivefile: %s, MaxFileSize: %v, MaxRolls: %v",
		rws.FileName,
		RollingArchiveTypesStringRepresentation[rws.ArchiveType],
		rws.ArchivePath,
		rws.MaxFileSize,
		rws.MaxRolls)
}

// --------------------------------------------------
//      Rolling writer by TIME
// --------------------------------------------------

// RollingFileWriterTime performs roll when a specified time Interval has passed.
type RollingFileWriterTime struct {
	*RollingFileWriter
	TimePattern         string
	Interval            RollingIntervalType
	CurrentTimeFileName string
}

func NewRollingFileWriterTime(fpath string, atype RollingArchiveType, apath string, maxr int,
	TimePattern string, Interval RollingIntervalType) (*RollingFileWriterTime, error) {

	rw, err := NewRollingFileWriter(fpath, RollingTypeTime, atype, apath, maxr)
	if err != nil {
		return nil, err
	}
	rws := &RollingFileWriterTime{rw, TimePattern, Interval, ""}
	rws.Self = rws
	return rws, nil
}

func (rwt *RollingFileWriterTime) needsToRoll() (bool, error) {
	if rwt.OriginalFileName+rollingLogHistoryDelimiter+time.Now().Format(rwt.TimePattern) == rwt.FileName {
		return false, nil
	}
	if rwt.Interval == RollingIntervalAny {
		return true, nil
	}

	tprev, err := time.ParseInLocation(rwt.TimePattern, rwt.getFileTail(rwt.FileName), time.Local)
	if err != nil {
		return false, err
	}

	diff := time.Now().Sub(tprev)
	switch rwt.Interval {
	case RollingIntervalDaily:
		return diff >= 24*time.Hour, nil
	}
	return false, fmt.Errorf("unknown Interval type: %d", rwt.Interval)
}

func (rwt *RollingFileWriterTime) isFileTailValid(tail string) bool {
	if len(tail) == 0 {
		return false
	}
	_, err := time.ParseInLocation(rwt.TimePattern, tail, time.Local)
	return err == nil
}

type rollTimeFileTailsSlice struct {
	data    []string
	pattern string
}

func (p rollTimeFileTailsSlice) Len() int { return len(p.data) }
func (p rollTimeFileTailsSlice) Less(i, j int) bool {
	t1, _ := time.ParseInLocation(p.pattern, p.data[i], time.Local)
	t2, _ := time.ParseInLocation(p.pattern, p.data[j], time.Local)
	return t1.Before(t2)
}
func (p rollTimeFileTailsSlice) Swap(i, j int) { p.data[i], p.data[j] = p.data[j], p.data[i] }

func (rwt *RollingFileWriterTime) sortFileTailsAsc(fs []string) ([]string, error) {
	ss := rollTimeFileTailsSlice{data: fs, pattern: rwt.TimePattern}
	sort.Sort(ss)
	return ss.data, nil
}

func (rwt *RollingFileWriterTime) getNewHistoryFileNameTail(lastRollFileTail string) string {
	return ""
}

func (rwt *RollingFileWriterTime) getCurrentModifiedFileName(OriginalFileName string) string {
	return OriginalFileName + rollingLogHistoryDelimiter + time.Now().Format(rwt.TimePattern)
}

func (rwt *RollingFileWriterTime) String() string {
	return fmt.Sprintf("Rolling file writer (By TIME): filename: %s, archive: %s, archivefile: %s, maxInterval: %v, pattern: %s, MaxRolls: %v",
		rwt.FileName,
		RollingArchiveTypesStringRepresentation[rwt.ArchiveType],
		rwt.ArchivePath,
		rwt.Interval,
		rwt.TimePattern,
		rwt.MaxRolls)
}

// getDirFilePaths return full paths of the files located in the directory.
// Remark: Ignores files for which fileFilter returns false.
func getDirFilePaths(dirPath string, fpFilter filePathFilter, pathIsName bool) ([]string, error) {
	dfi, err := os.Open(dirPath)
	if err != nil {
		return nil, newCannotOpenFileError("Cannot open directory " + dirPath)
	}
	defer dfi.Close()

	var absDirPath string
	if !filepath.IsAbs(dirPath) {
		absDirPath, err = filepath.Abs(dirPath)
		if err != nil {
			return nil, fmt.Errorf("cannot get absolute path of directory: %s", err.Error())
		}
	} else {
		absDirPath = dirPath
	}

	// TODO: check if dirPath is really directory.
	// Size of read buffer (i.e. chunk of items read at a time).
	rbs := 2 << 5
	filePaths := []string{}

	var fp string
L:
	for {
		// Read directory entities by reasonable chuncks
		// to prevent overflows on big number of files.
		fis, e := dfi.Readdir(rbs)
		switch e {
		// It's OK.
		case nil:
		// Do nothing, just continue cycle.
		case io.EOF:
			break L
		// Indicate that something went wrong.
		default:
			return nil, e
		}
		// THINK: Maybe, use async running.
		for _, fi := range fis {
			// NB: Should work on every Windows and non-Windows OS.
			if isRegular(fi.Mode()) {
				if pathIsName {
					fp = fi.Name()
				} else {
					// Build full path of a file.
					fp = filepath.Join(absDirPath, fi.Name())
				}
				// Check filter condition.
				if fpFilter != nil && !fpFilter(fp) {
					continue
				}
				filePaths = append(filePaths, fp)
			}
		}
	}
	return filePaths, nil
}

// filePathFilter is a filtering creteria function for file path.
// Must return 'false' to set aside the given file.
type filePathFilter func(filePath string) bool

func newCannotOpenFileError(fname string) *cannotOpenFileError {
	return &cannotOpenFileError{baseError{message: "Cannot open file: " + fname}}
}

type cannotOpenFileError struct {
	baseError
}

// Base struct for custom errors.
type baseError struct {
	message string
}

func (be baseError) Error() string {
	return be.message
}

func isRegular(m os.FileMode) bool {
	return m&os.ModeType == 0
}
