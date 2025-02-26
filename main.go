package main

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strconv"
	"strings"

	excelize "github.com/360EntSecGroup-Skylar/excelize/v2"
	"github.com/skratchdot/open-golang/open"
	"github.com/sqweek/dialog"
)

const SheetName string = "Sheet1"
const LOG_PARSED string = "./log-parsed.xlsx"

const ALPHABET string = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"

func main() {
	logPath, pathErr := dialog.File().Title("请选择日志文件").Filter("Log/Text file", "log", "txt").Load()
	if pathErr != nil {
		logPath = "./222.log"
	}

	file, readErr := ioutil.ReadFile(logPath)
	if readErr == nil {
		content := string(file)
		mainThread := matchMainThread(content)
		subThread := matchSubThread(content)

		createTable(mainThread, subThread)

		openErr := open.Start(LOG_PARSED)
		if openErr != nil {
			open.StartWith(LOG_PARSED, "Microsoft Excel")
		}
	}
}

// mainThread { timeKey: { date: date, total: total } }
func matchMainThread(file string) map[string]map[string]string {
	mainTread := map[string]map[string]string{}

	timeKeyReg := regexp.MustCompile(`MAIN_(\w+)\]\s+time:(\d{4}-\d{2}-\d{2}\s\d{2}:\d{2}:\d{2}.\d{3})`)
	timeKeyMatched := timeKeyReg.FindAllStringSubmatch(file, -1)
	if len(timeKeyMatched) == 0 {
		return mainTread
	}

	timeKeyTotalLineReg := regexp.MustCompile(`.*MAIN_TIME.*\s(\S+)`)
	timeKeyTotalLineMatched := timeKeyTotalLineReg.FindStringSubmatch(file)

	timeKeyTotalReg := regexp.MustCompile(`(\w+):(\d+)`)
	timeKeyTotalMatched := timeKeyTotalReg.FindAllStringSubmatch(timeKeyTotalLineMatched[1], -1)

	for _, keyDate := range timeKeyMatched {
		key, date := keyDate[1], keyDate[2]
		mainTread[key] = map[string]string{"date": date}
	}

	for _, keyTotal := range timeKeyTotalMatched {
		key, total := keyTotal[1], keyTotal[2]
		if _, ok := mainTread[key]; ok {
			mainTread[key]["total"] = total
		} else {
			mainTread[key] = map[string]string{"total": total}
		}
	}

	return mainTread
}

// subThread { ruleID: { timeKey: { date: date, total: total } } }
func matchSubThread(file string) map[string]map[string]map[string]string {
	timeKeyReg := regexp.MustCompile(`SUB_(\w+)_(\w+)\]\s+time:(\d{4}-\d{2}-\d{2}\s\d{2}:\d{2}:\d{2}.\d{3})`)
	timeKeyMatched := timeKeyReg.FindAllStringSubmatch(file, -1)

	subThread := map[string]map[string]map[string]string{}
	for _, ruleIDKeyDate := range timeKeyMatched {
		ruleID, key, date := ruleIDKeyDate[1], ruleIDKeyDate[2], ruleIDKeyDate[3]
		if _, ruleIDOk := subThread[ruleID]; ruleIDOk {
			if _, keyOk := subThread[ruleID][key]; !keyOk {
				subThread[ruleID][key] = map[string]string{"date": date}
			}
		} else {
			subThread[ruleID] = map[string]map[string]string{key: {"date": date}}
		}
	}

	timeTotalKeyLineReg := regexp.MustCompile(`.*SUB_TIME_(\d+)\S+\s(.*)`)
	timeTotalKeyLineMatched := timeTotalKeyLineReg.FindAllStringSubmatch(file, -1)

	timeKeyTotalReg := regexp.MustCompile(`(\w+):([\d\w]+)`)
	for _, keyTotalLineMatched := range timeTotalKeyLineMatched {
		ruleID, keyTotalLine := keyTotalLineMatched[1], keyTotalLineMatched[2]
		timeKeyTotalMatched := timeKeyTotalReg.FindAllStringSubmatch(keyTotalLine, -1)
		for _, keyTotal := range timeKeyTotalMatched {
			key, total := keyTotal[1], keyTotal[2]
			if _, ok := subThread[ruleID][key]; ok {
				subThread[ruleID][key]["total"] = total
			} else {
				subThread[ruleID][key] = map[string]string{"total": total}
			}
		}
	}

	return subThread
}

func createTable(mainThread map[string]map[string]string, subThread map[string]map[string]map[string]string) {

	f := excelize.NewFile()

	createMainTable(f, mainThread)
	createSubTable(f, subThread)

	err := f.SaveAs(LOG_PARSED)
	if err != nil {
		fmt.Println("保存失败: ", err)
	}
}

func createMainTable(f *excelize.File, mainThread map[string]map[string]string) {

	COL_NAME := strings.Split(ALPHABET, "")

	for row, header := range MainHeaders {
		var date, total string
		date = mainThread[header.Key]["date"]
		if date == "" {
			date = mainThread[header.Key+"Time"]["date"]
		}
		total = mainThread[header.Key]["total"]
		if total == "" {
			total = mainThread[header.Key+"Time"]["total"]
		}

		var cols = []string{header.Name, total, date}
		for i := 0; i < 3; i++ {
			f.SetCellValue(SheetName, COL_NAME[i]+strconv.Itoa(row+1), cols[i])
		}
	}
}

func createSubTable(f *excelize.File, subThread map[string]map[string]map[string]string) {

	COL_NAME := strings.Split(ALPHABET, "")
	row := 20

	collections := map[string][]int{}
	for ruleID := range subThread {
		f.SetCellValue(SheetName, COL_NAME[0]+strconv.Itoa(row), ruleID)
		for idx, col := range SubHeaders {
			val := subThread[ruleID][col]["total"]
			if val == "" {
				val = subThread[ruleID][col]["date"]
			} else {
				total, err := strconv.Atoi(val)
				if err == nil {
					if _, ok := collections[col]; ok {
						collections[col] = append(collections[col], total)
					} else {
						collections[col] = []int{total}
					}
				}
			}
			f.SetCellValue(SheetName, COL_NAME[idx+1]+strconv.Itoa(row), val)
		}

		row++
	}
	createStats(f, collections)
}

func createStats(f *excelize.File, collections map[string][]int) {
	COL_NAME := strings.Split(ALPHABET, "")
	row := 19
	f.SetCellValue(SheetName, COL_NAME[0]+strconv.Itoa(row), "RuleID")
	statsHeaders := []string{"max", "min", "avg", "mid", "more"}
	for sidx, statsKey := range statsHeaders {
		f.SetCellValue(SheetName, COL_NAME[0]+strconv.Itoa(row-1-sidx), statsKey)
	}
	for kidx, timeKey := range SubHeaders {
		f.SetCellValue(SheetName, COL_NAME[kidx+1]+strconv.Itoa(row), timeKey)
		stats := getStats(collections[timeKey])
		for sidx, statsKey := range statsHeaders {
			f.SetCellValue(SheetName, COL_NAME[kidx+1]+strconv.Itoa(row-1-sidx), stats[statsKey])
		}
	}
}

func getStats(arr []int) map[string]int {
	if len(arr) == 0 {
		return map[string]int{}
	}
	numMap := map[int]int{}
	avg := 0
	for _, val := range arr {
		avg += val
		if num, ok := numMap[val]; ok {
			numMap[val] = num + 1
		} else {
			numMap[val] = 1
		}
	}
	avg = avg / len(arr)

	more, length := 0, 0
	for key, val := range numMap {
		if val >= length {
			length = val
			more = key
		}
	}

	sorted := quickSort(arr)

	return map[string]int{
		"max":  sorted[len(sorted)-1],
		"min":  sorted[0],
		"avg":  avg,
		"mid":  sorted[len(sorted)/2],
		"more": more,
	}
}

func quickSort(arr []int) []int {
	if len(arr) <= 1 {
		return arr
	}

	midIndex := len(arr) / 2
	mid := arr[midIndex]
	arr = append(arr[:midIndex], arr[midIndex+1:]...)

	var leftArr, rightArr []int
	for _, val := range arr {
		if val < mid {
			leftArr = append(leftArr, val)
		} else {
			rightArr = append(rightArr, val)
		}
	}

	result := append(append(quickSort(leftArr), mid), quickSort(rightArr)...)
	return result
}
