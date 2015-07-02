package qfetch

import (
	"bufio"
	"fmt"
	"github.com/qiniu/api/auth/digest"
	"github.com/qiniu/api/rs"
	"github.com/syndtr/goleveldb/leveldb"
	"os"
	"strings"
	"sync"
)

func Fetch(job, filePath, bucket, accessKey, secretKey string, worker int) {
	//open file
	fh, openErr := os.Open(filePath)
	if openErr != nil {
		fmt.Println("Open resource file error,", openErr)
		return
	}
	defer fh.Close()

	//open leveldb
	proFile := fmt.Sprintf(".%s.job", job)
	ldb, lerr := leveldb.OpenFile(proFile, nil)

	if lerr != nil {
		fmt.Println("Open fetch progress file error,", lerr)
		return
	}
	defer ldb.Close()
	//fetch prepare
	mac := digest.Mac{
		accessKey, []byte(secretKey),
	}
	client := rs.New(&mac)
	wg := sync.WaitGroup{}
	var seq int = 0
	//scan each line
	bReader := bufio.NewScanner(fh)
	bReader.Split(bufio.ScanLines)
	for bReader.Scan() {
		line := strings.TrimSpace(bReader.Text())
		if line == "" {
			continue
		}

		items := strings.Split(line, "\t")
		if len(items) != 2 {
			fmt.Println("Invalid resource line,", line)
			continue
		}

		resUrl := items[0]
		resKey := items[1]

		//check from leveldb whether it is done
		val, exists := ldb.Get([]byte(resUrl), nil)
		if exists == nil && string(val) == resKey {
			continue
		}

		//otherwise fetch it
		seq += 1
		if seq%worker == 0 {
			wg.Wait()
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			fErr := client.Fetch(nil, resUrl, bucket, resKey)
			if fErr == nil {
				ldb.Put([]byte(resUrl), []byte(resKey), nil)
			} else {
				fmt.Println("Fetch", resUrl, " error due to", fErr)
			}
		}()
	}
	wg.Wait()
}