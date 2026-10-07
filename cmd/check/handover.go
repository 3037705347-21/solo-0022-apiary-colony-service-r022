package main

import (
	"encoding/json"
	"fmt"
	"sync"
)

// checkHandoverKeeper 校验值守人交接：交接前后记录归属、交接历史可读，
// 以及并发交接与写入交错时每条记录只归属一位值守人。
func checkHandoverKeeper(c *client) error {
	if err := seedApiary(c, demoApiaryID, "北岭蜂场", "北岭片区", "周岚"); err != nil {
		return err
	}
	// 交接前安装的蜂箱与提交的巡检归属建档值守人。
	if _, err := c.expect("POST", "/apiaries/apiary-north/hives", map[string]any{
		"id": "hive-01", "model": "langstroth",
	}, 201, `"keeper":"周岚"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/hive-01/inspections", map[string]any{
		"observedAt": "2026-04-10T09:00:00Z", "colonyStrength": 4, "broodPattern": "solid",
		"temper": "calm", "miteCount": 3, "note": "交接前巡检",
	}, 201, `"keeper":"周岚"`); err != nil {
		return err
	}
	// 失败路径：接任者为空、蜂场不存在。
	if _, err := c.expect("POST", "/apiaries/apiary-north/keeper-handovers", map[string]any{
		"toKeeper": "  ",
	}, 400, `"code":"invalid_request"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/apiaries/apiary-missing/keeper-handovers", map[string]any{
		"toKeeper": "李澈",
	}, 404, `"code":"not_found"`); err != nil {
		return err
	}
	// 办理交接，记录双方与交接时间。
	if _, err := c.expect("POST", "/apiaries/apiary-north/keeper-handovers", map[string]any{
		"toKeeper": "李澈",
	}, 201, `"handover-000001"`, `"fromKeeper":"周岚"`, `"toKeeper":"李澈"`, `"handedOverAt"`); err != nil {
		return err
	}
	// 接任者与现任相同视为冲突。
	if _, err := c.expect("POST", "/apiaries/apiary-north/keeper-handovers", map[string]any{
		"toKeeper": "李澈",
	}, 409, `"code":"conflict"`); err != nil {
		return err
	}
	// 蜂场当前值守人已切换。
	if _, err := c.expect("GET", "/apiaries/apiary-north", nil, 200, `"keeper":"李澈"`); err != nil {
		return err
	}
	// 交接后安装的蜂箱与提交的巡检归属接任值守人。
	if _, err := c.expect("POST", "/apiaries/apiary-north/hives", map[string]any{
		"id": "hive-02", "model": "warre",
	}, 201, `"keeper":"李澈"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/hive-01/inspections", map[string]any{
		"observedAt": "2026-04-12T09:00:00Z", "colonyStrength": 5, "broodPattern": "solid",
		"temper": "nervous", "miteCount": 2, "note": "交接后巡检",
	}, 201, `"keeper":"李澈"`); err != nil {
		return err
	}
	// 交接前形成的记录保持原归属可读。
	if _, err := c.expect("GET", "/hives/hive-01", nil, 200, `"keeper":"周岚"`); err != nil {
		return err
	}
	inspections, err := c.expect("GET", "/hives/hive-01/inspections", nil, 200,
		`"keeper":"周岚"`, `"keeper":"李澈"`)
	if err != nil {
		return err
	}
	if err := inspections.mustOrder(`"keeper":"周岚"`, `"keeper":"李澈"`); err != nil {
		return err
	}
	// 交接历史可读。
	if _, err := c.expect("GET", "/apiaries/apiary-north/keeper-handovers", nil, 200,
		`"handover-000001"`, `"fromKeeper":"周岚"`, `"toKeeper":"李澈"`); err != nil {
		return err
	}
	if _, err := c.expect("GET", "/apiaries/apiary-missing/keeper-handovers", nil, 404,
		`"code":"not_found"`); err != nil {
		return err
	}
	return checkHandoverRace(c)
}

// checkHandoverRace 让连续交接与蜂箱安装、巡检提交并发交错，随后在同一份
// 交接链上验证：交接记录首尾相接、当前值守人等于链尾，且每条蜂箱与巡检
// 记录归属且仅归属链上出现过的某一位值守人。
func checkHandoverRace(c *client) error {
	if err := seedApiary(c, "apiary-race", "河口蜂场", "河口片区", "周岚"); err != nil {
		return err
	}
	if err := seedHive(c, "apiary-race", "hive-r-01", "langstroth"); err != nil {
		return err
	}
	keepers := []string{"李澈", "王蔷", "赵岷", "陈序", "刘蔚", "孙澄"}
	var wg sync.WaitGroup
	errs := make(chan error, 3)
	wg.Add(3)
	go func() {
		defer wg.Done()
		for _, keeper := range keepers {
			if _, err := c.expect("POST", "/apiaries/apiary-race/keeper-handovers", map[string]any{
				"toKeeper": keeper,
			}, 201, `"toKeeper":"`+keeper+`"`); err != nil {
				errs <- err
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := 2; i <= 9; i++ {
			if err := seedHive(c, "apiary-race", fmt.Sprintf("hive-r-%02d", i), "top_bar"); err != nil {
				errs <- err
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 6; i++ {
			if _, err := c.expect("POST", "/hives/hive-r-01/inspections", map[string]any{
				"colonyStrength": 3, "broodPattern": "solid", "temper": "calm", "miteCount": 1,
			}, 201); err != nil {
				errs <- err
				return
			}
		}
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			return err
		}
	}

	reply, err := c.expect("GET", "/apiaries/apiary-race/keeper-handovers", nil, 200)
	if err != nil {
		return err
	}
	var handovers []struct {
		ID         string `json:"id"`
		FromKeeper string `json:"fromKeeper"`
		ToKeeper   string `json:"toKeeper"`
	}
	if err := json.Unmarshal(reply.body, &handovers); err != nil {
		return fmt.Errorf("交接历史不是合法 JSON：%v", err)
	}
	if len(handovers) != len(keepers) {
		return fmt.Errorf("交接记录数量应为 %d，实际 %d：%s", len(keepers), len(handovers), reply.text())
	}
	allowed := map[string]bool{"周岚": true}
	previous := "周岚"
	for i, handover := range handovers {
		if handover.FromKeeper != previous {
			return fmt.Errorf("交接链在第 %d 条断裂：交出方 %q 与上一任 %q 不符", i+1, handover.FromKeeper, previous)
		}
		if i > 0 && handovers[i-1].ID >= handover.ID {
			return fmt.Errorf("交接编号未按序递增：%q 之后出现 %q", handovers[i-1].ID, handover.ID)
		}
		allowed[handover.ToKeeper] = true
		previous = handover.ToKeeper
	}
	if _, err := c.expect("GET", "/apiaries/apiary-race", nil, 200, `"keeper":"`+previous+`"`); err != nil {
		return err
	}

	for i := 1; i <= 9; i++ {
		hiveID := fmt.Sprintf("hive-r-%02d", i)
		found, err := c.expect("GET", "/hives/"+hiveID, nil, 200)
		if err != nil {
			return err
		}
		var hive struct {
			Keeper string `json:"keeper"`
		}
		if err := json.Unmarshal(found.body, &hive); err != nil {
			return fmt.Errorf("蜂箱 %s 响应不是合法 JSON：%v", hiveID, err)
		}
		if !allowed[hive.Keeper] {
			return fmt.Errorf("蜂箱 %s 归属值守人 %q 不在交接链上", hiveID, hive.Keeper)
		}
	}

	list, err := c.expect("GET", "/hives/hive-r-01/inspections", nil, 200)
	if err != nil {
		return err
	}
	var records []struct {
		ID     string `json:"id"`
		Keeper string `json:"keeper"`
	}
	if err := json.Unmarshal(list.body, &records); err != nil {
		return fmt.Errorf("巡检列表不是合法 JSON：%v", err)
	}
	if len(records) != 6 {
		return fmt.Errorf("巡检数量应为 6，实际 %d：%s", len(records), list.text())
	}
	for _, record := range records {
		if !allowed[record.Keeper] {
			return fmt.Errorf("巡检 %s 归属值守人 %q 不在交接链上", record.ID, record.Keeper)
		}
	}
	return nil
}
