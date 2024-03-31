package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"io"
	"log"
	"net/http"
	"strings"
)

var DB *sql.DB

const (
	USERNAME = "root"
	PASSWORD = "123456"
	HOST     = "127.0.0.1"
	PORT     = "3306"
	DBNAME   = "86"
)

// https://api.bilibili.com/x/v2/reply/wbi/main?oid=18168483&type=1&mode=3&pagination_str=%7B%22offset%22:%22%22%7D&plat=1&web_location=1315875&w_rid=c1ba468346f3738d6820b2f53b113efb&wts=1711727275

type Data struct {
	Id       int64
	Comment  string
	Level    int64
	ParentId sql.NullInt64
}

type Comment struct {
	Code int64 `json:"code"`
	Data struct {
		Replies []struct {
			Content struct {
				JumpURL struct{}      `json:"jump_url"`
				MaxLine int64         `json:"max_line"`
				Members []interface{} `json:"members"`
				Message string        `json:"message"`
			} `json:"content"`
			Count  int64 `json:"count"`
			Folder struct {
				HasFolded bool   `json:"has_folded"`
				IsFolded  bool   `json:"is_folded"`
				Rule      string `json:"rule"`
			} `json:"folder"`
			Like    int64 `json:"like"`
			Replies []struct {
				Action  int64 `json:"action"`
				Assist  int64 `json:"assist"`
				Attr    int64 `json:"attr"`
				Content struct {
					JumpURL struct{} `json:"jump_url"`
					MaxLine int64    `json:"max_line"`
					Message string   `json:"message"`
				} `json:"content"`
				Rcount  int64       `json:"rcount"`
				Replies interface{} `json:"replies"`
			} `json:"replies"`
			Type int64 `json:"type"`
		} `json:"replies"`
	} `json:"data"`
	Message string `json:"message"`
}

func main() {
	InitDb()
	Spiders()
}

func Spiders() {
	//1. 发送请求

	//构造客户端
	client := http.Client{}
	req, err := http.NewRequest("GET", "https://api.bilibili.com/x/v2/reply/wbi/main?oid=298493034&type=1&mode=3&pagination_str=%7B%22offset%22:%22%22%7D&plat=1&seek_rpid=&web_location=1315875&w_rid=715ff9525616460321277400f517f400&wts=1711727804", nil)
	if err != nil {
		fmt.Println("req err", err)
	}
	//防止浏览器检测爬虫访问，所以加一些请求头，伪造成浏览器访问。
	resp, err := client.Do(req)
	//2. 解析网页
	//把它的一些信息读取出来
	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("io err", err)
	}

	//3. 获取节点信息
	var resultList Comment

	_ = json.Unmarshal(bodyText, &resultList)
	for _, result := range resultList.Data.Replies {
		// 创建一级评论对象并插入数据库
		comment1 := Data{
			Comment: result.Content.Message,
			Level:   1,
			ParentId: sql.NullInt64{
				Int64: 0,
				Valid: false,
			},
		}
		id, err := insertData(comment1)
		if err != nil {
			log.Fatal(err)
		}
		//fmt.Println("一级评论", result.Content.Message)

		for _, reply := range result.Replies {
			// 创建二级评论对象并插入数据库
			comment2 := Data{
				Comment: reply.Content.Message,
				Level:   2,
				ParentId: sql.NullInt64{
					Int64: id,
					Valid: true,
				},
			}
			_, err := insertData(comment2)
			if err != nil {
				log.Fatal(err)
			}
			//fmt.Println("二级评论", reply.Content.Message)
		}
	}
}

func InitDb() {
	path := strings.Join([]string{USERNAME, ":", PASSWORD, "@tcp(", HOST, ":", PORT, ")/", DBNAME, "?charset=utf8mb4&parseTime=True&loc=Local"}, "")
	db, err := sql.Open("mysql", path)
	if err != nil {
		log.Fatal(err)
	}

	db.SetConnMaxLifetime(10)
	db.SetMaxIdleConns(5)

	if err := db.Ping(); err != nil {
		fmt.Println("open database failed")
		log.Fatal(err)
	}
	fmt.Println("connect success")
	DB = db

}

func insertData(comment Data) (int64, error) {
	tx, err := DB.Begin()
	if err != nil {
		log.Fatal("begin err", err)
	}
	stmt, err := DB.Prepare("INSERT INTO comment(comment, level, parent_id) VALUES(?, ?, ?)")
	if err != nil {
		log.Fatal("prepare fail err", err)
	}
	defer stmt.Close()

	res, err := stmt.Exec(comment.Comment, comment.Level, comment.ParentId)
	if err != nil {
		log.Fatal("exec fail", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		log.Fatal(err)
	}
	_ = tx.Commit()
	return id, nil
}
