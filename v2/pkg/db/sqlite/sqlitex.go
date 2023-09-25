package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	db2 "github.com/zan8in/afrog/v2/pkg/db"
	"github.com/zan8in/afrog/v2/pkg/result"
	"github.com/zan8in/gologger"
)

var dbx *sqlx.DB

func InitX() error {

	err := initDBX()
	if err != nil || dbx == nil {
		return fmt.Errorf("error initializing databasex: %v", err)
	}

	insertChannel = make(chan *result.Result)

	wg.Add(1)
	go saveToDatabaseX()

	return nil
}

func SetResultX(result *result.Result) {
	insertChannel <- result
}

func saveToDatabaseX() {
	defer wg.Done()

	var wgAddx sync.WaitGroup
	for r := range insertChannel {
		wgAddx.Add(1)

		go func(r *result.Result) {
			defer wgAddx.Done()
			if err := addx(r); err != nil {
				gologger.Error().Msgf("Error inserting result into database: %v\n", err)
			}
		}(r)
	}
	wgAddx.Wait()
}

func initDBX() error {
	dbx = sqlx.MustConnect("sqlite3", "file:"+db2.GetSqliteFullDBName()+".db?cache=shared&mode=rwc&_journal_mode=WAL")

	// 设置连接池参数（可选）
	dbx.SetMaxOpenConns(50) // 设置最大打开连接数
	dbx.SetMaxIdleConns(25) // 设置最大空闲连接数

	_, err := dbx.Exec(db2.SqliteCreate)
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("error creating table: %v", err)
	}

	return dbx.Ping()
}

func CloseX() {
	select {
	case r, ok := <-insertChannel:
		if ok {
			if err := addx(r); err != nil {
				gologger.Error().Msgf("Error inserting result into database: %v\n", err)
			}
		}
	default:
		if insertChannel != nil {
			close(insertChannel)
		}
	}

	wg.Wait()

	if dbx != nil {
		dbx.Close()
	}
}

func addx(r *result.Result) error {
	insertSQL := "INSERT INTO result(taskid, vulid, vulname, target, fulltarget, severity, poc, result, created, fingerprint, extractor) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"

	currentTime := time.Now()
	createdTime := currentTime.Format("2006-01-02 15:04:05")

	poc, _ := json.Marshal(r.PocInfo)

	pocList := []db2.PocResult{}
	if len(r.AllPocResult) > 0 {
		for _, pocResult := range r.AllPocResult {
			var reqRaw []byte
			var respRaw []byte
			if pocResult != nil && pocResult.ResultRequest != nil && pocResult.ResultRequest.Raw != nil {
				reqRaw = pocResult.ResultRequest.Raw
			}
			if pocResult != nil && pocResult.ResultResponse != nil && pocResult.ResultResponse.Raw != nil {
				respRaw = pocResult.ResultResponse.Raw
			}
			pocList = append(pocList, db2.PocResult{
				Request:  string(reqRaw),
				Response: string(respRaw),
			})
		}
	}
	result, _ := json.Marshal(pocList)

	extractor, _ := json.Marshal(r.Extractor)

	finger, _ := json.Marshal(r.FingerResult)

	_, err := dbx.Exec(insertSQL, db2.TaskID, r.PocInfo.Id, r.PocInfo.Info.Name, r.Target, r.FullTarget, r.PocInfo.Info.Severity, poc, result, createdTime, finger, extractor)
	return err
}

func SelectX(severity, keyword, page string) ([]db2.ResultData, error) {

	var err error
	var query string
	var data []db2.ResultData

	// 计算 OFFSET，即从哪一行开始
	pageSize, err := strconv.Atoi(db2.LIMIT)
	if err != nil {
		pageSize = 100
	}
	pageInt, err := strconv.Atoi(page)
	if err != nil {
		pageInt = 1
	}
	offset := (pageInt - 1) * pageSize

	ctx := context.Background() // 使用默认上下文

	if len(keyword) == 0 && len(severity) == 0 {
		query := "SELECT * FROM " + db2.TableName + " ORDER BY id DESC LIMIT " + db2.LIMIT + " OFFSET ?"
		err = dbx.SelectContext(ctx, &data, query, offset)
		if err != nil {
			return nil, err
		}
	} else if len(keyword) > 0 && len(severity) == 0 {
		query = "SELECT * FROM " + db2.TableName + " WHERE vulid LIKE ? OR vulname LIKE ? ORDER BY id DESC LIMIT " + db2.LIMIT + " OFFSET ?"
		err = dbx.SelectContext(ctx, &data, query, "%"+keyword+"%", "%"+keyword+"%", offset)
		if err != nil {
			return nil, err
		}
	} else if len(keyword) > 0 && len(severity) > 0 {
		list := strings.Split(severity, ",")
		if len(list) == 1 {
			query = "SELECT * FROM " + db2.TableName + " WHERE severity = ? AND (vulid LIKE ? OR vulname LIKE ?) ORDER BY id DESC LIMIT " + db2.LIMIT + " OFFSET ?"
			err = dbx.SelectContext(ctx, &data, query, list[0], "%"+keyword+"%", "%"+keyword+"%", offset)
		} else if len(list) == 2 {
			query = "SELECT * FROM " + db2.TableName + " WHERE severity in (?,?) AND (vulid LIKE ? OR vulname LIKE ?)  ORDER BY id DESC LIMIT " + db2.LIMIT + " OFFSET ?"
			err = dbx.SelectContext(ctx, &data, query, list[0], list[1], "%"+keyword+"%", "%"+keyword+"%", offset)
		} else if len(list) == 3 {
			query = "SELECT * FROM " + db2.TableName + " WHERE severity in (?,?,?) AND (vulid LIKE ? OR vulname LIKE ?) ORDER BY id DESC LIMIT " + db2.LIMIT + " OFFSET ?"
			err = dbx.SelectContext(ctx, &data, query, list[0], list[1], list[2], "%"+keyword+"%", "%"+keyword+"%", offset)
		} else if len(list) == 4 {
			query = "SELECT * FROM " + db2.TableName + " WHERE severity in (?,?,?,?) AND (vulid LIKE ? OR vulname LIKE ?) ORDER BY id DESC LIMIT " + db2.LIMIT + " OFFSET ?"
			err = dbx.SelectContext(ctx, &data, query, list[0], list[1], list[2], list[3], "%"+keyword+"%", "%"+keyword+"%", offset)
		} else if len(list) == 5 {
			query = "SELECT * FROM " + db2.TableName + " ORDER BY id DESC LIMIT " + db2.LIMIT + " OFFSET ?"
			err = dbx.SelectContext(ctx, &data, query, offset)
		}
		if err != nil {
			return nil, err
		}
	} else if len(keyword) == 0 && len(severity) > 0 {
		list := strings.Split(severity, ",")
		if len(list) == 1 {
			query = "SELECT * FROM " + db2.TableName + " WHERE severity = ? ORDER BY id DESC LIMIT " + db2.LIMIT + " OFFSET ?"
			err = dbx.SelectContext(ctx, &data, query, list[0], offset)
		} else if len(list) == 2 {
			query = "SELECT * FROM " + db2.TableName + " WHERE severity in (?,?) ORDER BY id DESC LIMIT " + db2.LIMIT + " OFFSET ?"
			err = dbx.SelectContext(ctx, &data, query, list[0], list[1], offset)
		} else if len(list) == 3 {
			query = "SELECT * FROM " + db2.TableName + " WHERE severity in (?,?,?) ORDER BY id DESC LIMIT " + db2.LIMIT + " OFFSET ?"
			err = dbx.SelectContext(ctx, &data, query, list[0], list[1], list[2], offset)
		} else if len(list) == 4 {
			query = "SELECT * FROM " + db2.TableName + " WHERE severity in (?,?,?,?) ORDER BY id DESC LIMIT " + db2.LIMIT + " OFFSET ?"
			err = dbx.SelectContext(ctx, &data, query, list[0], list[1], list[2], list[3], offset)
		} else if len(list) == 5 {
			query = "SELECT * FROM " + db2.TableName + " ORDER BY id DESC LIMIT " + db2.LIMIT + " OFFSET ?"
			err = dbx.SelectContext(ctx, &data, query, offset)
		}
		if err != nil {
			return nil, err
		}
	}

	for key, item := range data {
		data[key].Severity = strings.ToUpper(item.Severity)

		// item.Result = strings.ReplaceAll(item.Result, "\n", "<br>")
		json.Unmarshal([]byte(item.Result), &data[key].ResultList)
		data[key].Result = ""
	}

	return data, nil
}

func Count() int64 {
	var count int64
	query := "SELECT COUNT(*) FROM " + db2.TableName
	err := dbx.Get(&count, query)
	if err != nil {
		return 0
	}
	return count
}
