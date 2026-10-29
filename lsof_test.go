package main

import "testing"

func TestLsofFilesPreservePathsAndExcludeNonFileDescriptors(t *testing.T) {
	data := []byte("p42\x00\nfcwd\x00tDIR\x00n/home/example\x00\nftxt\x00tREG\x00n/usr/bin/example\x00\nf3\x00tREG\x00n/tmp/a file\nwith-newline.db\x00\nf4\x00tIPv4\x00n127.0.0.1:80\x00\nf5\x00tDIR\x00n/tmp/folder\x00\n")
	files := parseLsofFiles(data)
	if len(files) != 2 || files[0].Fd != 3 || files[0].Path != "/tmp/a file\nwith-newline.db" || files[1].Fd != 5 || files[1].Path != "/tmp/folder" {
		t.Fatal("file metadata parsing failed")
	}
}
func TestLsofFilesIgnoreMalformedRecords(t *testing.T) {
	for _, data := range []string{"", "p42\x00\nf3\x00n/tmp/no-type\x00", "fwrong\x00tREG\x00n/tmp/file\x00", "f3\x00tREG\x00nrelative\x00"} {
		if len(parseLsofFiles([]byte(data))) != 0 {
			t.Fatal("malformed record accepted")
		}
	}
}
func TestLimitedCommandOutputReportsTruncation(t *testing.T) {
	output := &limitedOutput{remaining: 3}
	n, err := output.Write([]byte("12345"))
	if err != nil || n != 5 || !output.truncated || output.String() != "123" {
		t.Fatal("command output must remain bounded and report truncation")
	}
}
