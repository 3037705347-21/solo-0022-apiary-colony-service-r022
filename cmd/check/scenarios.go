package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

const (
	demoApiaryID = "apiary-north"
	demoHiveID   = "hive-01"
)

func seedApiary(c *client, id, name, district, keeper string) error {
	_, err := c.expect("POST", "/apiaries", map[string]any{
		"id":       id,
		"name":     name,
		"district": district,
		"keeper":   keeper,
	}, 201, `"id":"`+id+`"`)
	return err
}

func seedHive(c *client, apiaryID, hiveID, model string) error {
	_, err := c.expect("POST", "/apiaries/"+apiaryID+"/hives", map[string]any{
		"id":    hiveID,
		"model": model,
	}, 201, `"state":"colonized"`)
	return err
}

func seedDefault(c *client) error {
	if err := seedApiary(c, demoApiaryID, "北岭蜂场", "北岭片区", "周岚"); err != nil {
		return err
	}
	return seedHive(c, demoApiaryID, demoHiveID, "langstroth")
}

func checkRegisterApiary(c *client) error {
	if _, err := c.expect("POST", "/apiaries", map[string]any{
		"id": "apiary-north", "name": "北岭蜂场", "district": "北岭片区", "keeper": "周岚",
	}, 201, `"id":"apiary-north"`, `"name":"北岭蜂场"`); err != nil {
		return err
	}
	if _, err := c.expect("GET", "/apiaries/apiary-north", nil, 200,
		`"district":"北岭片区"`, `"keeper":"周岚"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/apiaries", map[string]any{
		"id": "apiary-north", "name": "重复蜂场", "district": "北岭片区", "keeper": "周岚",
	}, 409, `"code":"conflict"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/apiaries", map[string]any{
		"id": "apiary-south", "name": "", "district": "南坡片区", "keeper": "李澈",
	}, 400, `"code":"invalid_request"`); err != nil {
		return err
	}
	return nil
}

func checkHiveInstall(c *client) error {
	if err := seedApiary(c, demoApiaryID, "北岭蜂场", "北岭片区", "周岚"); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/apiaries/apiary-north/hives", map[string]any{
		"id": "hive-01", "model": "langstroth",
	}, 201, `"state":"colonized"`, `"apiaryId":"apiary-north"`); err != nil {
		return err
	}
	if _, err := c.expect("GET", "/hives/hive-01", nil, 200, `"model":"langstroth"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/apiaries/apiary-north/hives", map[string]any{
		"id": "hive-01", "model": "warre",
	}, 409, `"code":"conflict"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/apiaries/apiary-missing/hives", map[string]any{
		"id": "hive-09", "model": "warre",
	}, 404, `"code":"not_found"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/apiaries/apiary-north/hives", map[string]any{
		"id": "hive-02", "model": "dadant",
	}, 400, `"code":"invalid_request"`); err != nil {
		return err
	}
	return nil
}

func checkRecordInspection(c *client) error {
	if err := seedDefault(c); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/hive-01/inspections", map[string]any{
		"observedAt": "2026-04-10T09:00:00Z", "colonyStrength": 9, "broodPattern": "patchy",
		"temper": "calm", "miteCount": 3, "note": "",
	}, 400, `"code":"invalid_request"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/hive-01/inspections", map[string]any{
		"observedAt": "2026-04-10T09:00:00Z", "colonyStrength": 3, "broodPattern": "patchy",
		"temper": "aggressive", "miteCount": 12, "note": "西侧巢门观察",
	}, 201, `"inspection-000001"`, `"mite_load"`, `"temper_risk"`); err != nil {
		return err
	}
	if _, err := c.expect("GET", "/hives/hive-01/inspections", nil, 200, `"note":"西侧巢门观察"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/hive-01/state", map[string]any{"target": "retired"}, 200,
		`"to":"retired"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/hive-01/inspections", map[string]any{
		"observedAt": "2026-04-11T09:00:00Z", "colonyStrength": 3, "broodPattern": "solid",
		"temper": "calm", "miteCount": 1, "note": "",
	}, 409, `"code":"invalid_state"`); err != nil {
		return err
	}
	return nil
}

func checkReplaceQueen(c *client) error {
	if err := seedDefault(c); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/hive-01/queens", map[string]any{
		"origin": "bred", "introducedAt": "2026-04-02T08:00:00Z", "reason": "aging",
	}, 201, `"queen-000001"`, `"markColor":"white"`, `"state":"laying"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/hive-01/queens", map[string]any{
		"origin": "cells", "introducedAt": "2026-04-20T08:00:00Z", "reason": "aggression",
	}, 201, `"queen-000002"`, `"markColor":"white"`); err != nil {
		return err
	}
	if _, err := c.expect("GET", "/hives/hive-01/queens", nil, 200,
		`"queen-000001"`, `"superseded"`, `"queen-000002"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/hive-01/queens", map[string]any{
		"origin": "imported", "introducedAt": "2026-05-01T08:00:00Z", "reason": "aging",
	}, 400, `"code":"invalid_request"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/hive-01/state", map[string]any{"target": "retired"}, 200,
		`"to":"retired"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/hive-01/queens", map[string]any{
		"origin": "bred", "introducedAt": "2026-05-02T08:00:00Z", "reason": "loss",
	}, 409, `"code":"invalid_state"`); err != nil {
		return err
	}
	return nil
}

func checkAdvanceHiveState(c *client) error {
	if err := seedDefault(c); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/hive-01/state", map[string]any{"target": "production"}, 200,
		`"from":"colonized"`, `"to":"production"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/hive-01/state", map[string]any{"target": "colonized"}, 409,
		`"code":"invalid_state"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/hive-01/state", map[string]any{"target": "production"}, 409,
		`"code":"invalid_state"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/hive-01/state", map[string]any{"target": "retired"}, 200,
		`"to":"retired"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/hive-01/state", map[string]any{"target": "production"}, 409,
		`"code":"invalid_state"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/hive-missing/state", map[string]any{"target": "production"}, 404,
		`"code":"not_found"`); err != nil {
		return err
	}
	return nil
}

func checkReadApiaryStatus(c *client) error {
	if err := seedApiary(c, demoApiaryID, "北岭蜂场", "北岭片区", "周岚"); err != nil {
		return err
	}
	for _, hiveID := range []string{"hive-01", "hive-02", "hive-03"} {
		if err := seedHive(c, demoApiaryID, hiveID, "langstroth"); err != nil {
			return err
		}
	}
	if _, err := c.expect("POST", "/hives/hive-02/state", map[string]any{"target": "production"}, 200,
		`"to":"production"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/hive-01/inspections", map[string]any{
		"observedAt": "2026-04-10T09:00:00Z", "colonyStrength": 4, "broodPattern": "solid",
		"temper": "calm", "miteCount": 12, "note": "",
	}, 201, `"mite_load"`); err != nil {
		return err
	}
	report, err := c.expect("GET", "/apiaries/apiary-north/status?asOf=2026-04-12T00:00:00Z", nil, 200,
		`"hiveCount":3`, `"colonized":2`, `"production":1`, `"layingQueens":0`,
		`"keeper":"周岚"`, `"hiveId":"hive-01","keeper":"周岚"`)
	if err != nil {
		return err
	}
	for _, fragment := range []string{
		`"needsInspection":["hive-02","hive-03"]`,
		`"concernHives":["hive-01"]`,
	} {
		if !strings.Contains(report.text(), fragment) {
			return fmt.Errorf("状态汇总缺少 %q：%s", fragment, report.text())
		}
	}
	if err := report.mustOrder(`"hiveId":"hive-01"`, `"hiveId":"hive-02"`); err != nil {
		return err
	}
	if _, err := c.expect("GET", "/apiaries/apiary-missing/status", nil, 404, `"code":"not_found"`); err != nil {
		return err
	}
	if _, err := c.expect("GET", "/apiaries/apiary-north/status?asOf=not-a-time", nil, 400,
		`"code":"invalid_request"`); err != nil {
		return err
	}
	return nil
}

// keeperOrder 把值守人映射为交接链上的次序，用于断言归属单调推进、不回退。
func keeperOrder(name string) int {
	switch name {
	case "周岚":
		return 0
	case "李澈":
		return 1
	case "孙仲":
		return 2
	default:
		return -1
	}
}

func inspectionPayload(keeperSeq int) map[string]any {
	return map[string]any{
		"colonyStrength": 3, "broodPattern": "patchy",
		"temper": "calm", "miteCount": 1, "note": fmt.Sprintf("巡检-%d", keeperSeq),
	}
}

// checkHandoverKeeper 覆盖值守人交接：双方信息与时间留存、历史归属不变、
// 新蜂箱与新巡检按新值守人归属，以及并发交接/交接与巡检交错时单一归属的线性化结果。
func checkHandoverKeeper(c *client) error {
	if err := seedApiary(c, demoApiaryID, "北岭蜂场", "北岭片区", "周岚"); err != nil {
		return err
	}
	if err := seedHive(c, demoApiaryID, "hive-01", "langstroth"); err != nil {
		return err
	}
	// 交接前的蜂箱与巡检固化为原值守人。
	if _, err := c.expect("POST", "/hives/hive-01/inspections", inspectionPayload(1), 201,
		`"keeper":"周岚"`); err != nil {
		return err
	}

	// 协调人办理交接：声明双方，服务留存时间与双方信息。
	if _, err := c.expect("POST", "/apiaries/apiary-north/handovers", map[string]any{
		"fromKeeper": "周岚", "toKeeper": "李澈", "handedOverAt": "2026-05-01T08:00:00Z",
	}, 201, `"handover-000001"`, `"fromKeeper":"周岚"`, `"toKeeper":"李澈"`,
		`"handedOverAt":"2026-05-01T08:00:00Z"`); err != nil {
		return err
	}

	// 蜂场当前值守人切换；交接前形成的蜂箱与巡检归属保持原值、仍可读。
	if _, err := c.expect("GET", "/apiaries/apiary-north", nil, 200, `"keeper":"李澈"`); err != nil {
		return err
	}
	if _, err := c.expect("GET", "/hives/hive-01", nil, 200, `"keeper":"周岚"`); err != nil {
		return err
	}
	if _, err := c.expect("GET", "/hives/hive-01/inspections", nil, 200, `"keeper":"周岚"`); err != nil {
		return err
	}

	// 交接后新建蜂箱按新值守人归属；交接后提交的巡检也按新值守人归属。
	if _, err := c.expect("POST", "/apiaries/apiary-north/hives", map[string]any{
		"id": "hive-02", "model": "warre",
	}, 201, `"keeper":"李澈"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/hives/hive-01/inspections", inspectionPayload(2), 201,
		`"keeper":"李澈"`); err != nil {
		return err
	}

	// 状态汇总同时呈现蜂场当前值守人与各蜂箱固化归属。
	if _, err := c.expect("GET", "/apiaries/apiary-north/status", nil, 200,
		`"keeper":"李澈"`, `"hiveId":"hive-01","keeper":"周岚"`, `"hiveId":"hive-02","keeper":"李澈"`); err != nil {
		return err
	}

	// 交接链可读：再办一次交接，列表按时间升序保留双方与时间。
	if _, err := c.expect("POST", "/apiaries/apiary-north/handovers", map[string]any{
		"fromKeeper": "李澈", "toKeeper": "孙仲", "handedOverAt": "2026-06-01T08:00:00Z",
	}, 201, `"handover-000002"`); err != nil {
		return err
	}
	if _, err := c.expect("GET", "/apiaries/apiary-north/handovers", nil, 200,
		`"handover-000001"`, `"handover-000002"`); err != nil {
		return err
	}
	handovers, err := c.expect("GET", "/apiaries/apiary-north/handovers", nil, 200)
	if err != nil {
		return err
	}
	if err := handovers.mustOrder(`"handover-000001"`, `"handover-000002"`); err != nil {
		return err
	}

	// 失败路径：旧值守人重复发起、交接给本人、字段为空、未知蜂场。
	if _, err := c.expect("POST", "/apiaries/apiary-north/handovers", map[string]any{
		"fromKeeper": "周岚", "toKeeper": "李澈",
	}, 409, `"code":"conflict"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/apiaries/apiary-north/handovers", map[string]any{
		"fromKeeper": "孙仲", "toKeeper": "孙仲",
	}, 400, `"code":"invalid_request"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/apiaries/apiary-north/handovers", map[string]any{
		"fromKeeper": "孙仲", "toKeeper": "  ",
	}, 400, `"code":"invalid_request"`); err != nil {
		return err
	}
	if _, err := c.expect("GET", "/apiaries/apiary-missing/handovers", nil, 404,
		`"code":"not_found"`); err != nil {
		return err
	}

	// 并发交接：多名候选新值守人同时发起，只有一个提交成功，其余全部冲突。
	if err := seedApiary(c, "apiary-east", "东岭蜂场", "东岭片区", "周岚"); err != nil {
		return err
	}
	if err := seedHive(c, "apiary-east", "hive-11", "langstroth"); err != nil {
		return err
	}
	candidates := []string{"李澈", "孙仲", "陈岭", "韩秋", "赵禾", "吴越", "郑南", "王北", "冯一", "何二"}
	var wg sync.WaitGroup
	type result struct {
		status int
		body   string
	}
	results := make([]result, len(candidates))
	start := make(chan struct{})
	for i, candidate := range candidates {
		wg.Add(1)
		go func(index int, toKeeper string) {
			defer wg.Done()
			<-start
			reply, callErr := c.call("POST", "/apiaries/apiary-east/handovers", map[string]any{
				"fromKeeper": "周岚", "toKeeper": toKeeper,
			})
			if callErr != nil {
				results[index] = result{status: 0, body: callErr.Error()}
				return
			}
			results[index] = result{status: reply.status, body: reply.text()}
		}(i, candidate)
	}
	close(start)
	wg.Wait()

	var winner string
	wins := 0
	for index, res := range results {
		switch res.status {
		case 201:
			wins++
			var created struct {
				ToKeeper string `json:"toKeeper"`
			}
			if err := json.Unmarshal([]byte(res.body), &created); err != nil {
				return err
			}
			winner = created.ToKeeper
			if winner != candidates[index] {
				return fmt.Errorf("并发交接成功者应为候选 %q，实际 %q", candidates[index], winner)
			}
		case 409:
			if !strings.Contains(res.body, `"code":"conflict"`) {
				return fmt.Errorf("失败的并发交接应返回 conflict：%s", res.body)
			}
		default:
			return fmt.Errorf("并发交接只能是 201 或 409，候选 %q 得到 %d：%s", candidates[index], res.status, res.body)
		}
	}
	if wins != 1 {
		return fmt.Errorf("并发交接必须恰有一个成功，实际成功 %d 个", wins)
	}

	// 只有一名新值守人接管：当前值守人与随后新建的蜂箱都归该成功者。
	if _, err := c.expect("GET", "/apiaries/apiary-east", nil, 200, `"keeper":"`+winner+`"`); err != nil {
		return err
	}
	if _, err := c.expect("POST", "/apiaries/apiary-east/hives", map[string]any{
		"id": "hive-12", "model": "top_bar",
	}, 201, `"keeper":"`+winner+`"`); err != nil {
		return err
	}
	chain, err := c.expect("GET", "/apiaries/apiary-east/handovers", nil, 200)
	if err != nil {
		return err
	}
	if strings.Count(chain.text(), `"id":"handover-`) != 1 {
		return fmt.Errorf("并发交接只应留下一条记录：%s", chain.text())
	}

	// 交接与巡检交错：并发提交期间巡检归属按写锁提交次序单调推进，
	// 不会出现同一条巡检归属两名值守人，也不会回退到更早的值守人。
	if err := seedApiary(c, "apiary-west", "西岭蜂场", "西岭片区", "周岚"); err != nil {
		return err
	}
	if err := seedHive(c, "apiary-west", "hive-21", "langstroth"); err != nil {
		return err
	}
	const interleaved = 8
	type committed struct {
		id     string
		keeper string
		ok     bool
	}
	committedByGoroutine := make([]committed, interleaved)
	inspectStart := make(chan struct{})
	var inspectWG sync.WaitGroup
	for i := 0; i < interleaved; i++ {
		inspectWG.Add(1)
		go func(index int) {
			defer inspectWG.Done()
			<-inspectStart
			reply, callErr := c.call("POST", "/hives/hive-21/inspections", inspectionPayload(index+10))
			if callErr != nil || reply.status != 201 {
				return
			}
			var created struct {
				ID     string `json:"id"`
				Keeper string `json:"keeper"`
			}
			if json.Unmarshal(reply.body, &created) == nil {
				committedByGoroutine[index] = committed{id: created.ID, keeper: created.Keeper, ok: true}
			}
		}(i)
	}
	close(inspectStart)
	if _, err := c.call("POST", "/apiaries/apiary-west/handovers", map[string]any{
		"fromKeeper": "周岚", "toKeeper": "李澈",
	}); err != nil {
		return err
	}
	inspectWG.Wait()

	// 每条 201 响应只带一名合法值守人；按巡检编号（即仓储提交次序）排序后归属单调推进。
	commits := make([]committed, 0, interleaved)
	for index, item := range committedByGoroutine {
		if !item.ok {
			return fmt.Errorf("第 %d 条交错巡检未成功写入", index)
		}
		if keeperOrder(item.keeper) < 0 {
			return fmt.Errorf("交错巡检 %s 归属未知值守人 %q", item.id, item.keeper)
		}
		commits = append(commits, item)
	}
	sort.Slice(commits, func(i, j int) bool { return commits[i].id < commits[j].id })
	prevOrder := 0
	for _, item := range commits {
		if order := keeperOrder(item.keeper); order < prevOrder {
			return fmt.Errorf("按提交次序归属发生回退：%+v", commits)
		} else {
			prevOrder = order
		}
	}

	// 交错后按巡检编号（即提交次序）读回，归属只能在交接点由原值守人切到新值守人一次。
	listed, err := c.expect("GET", "/hives/hive-21/inspections", nil, 200)
	if err != nil {
		return err
	}
	var records []struct {
		Keeper string `json:"keeper"`
	}
	if err := json.Unmarshal(listed.body, &records); err != nil || len(records) != interleaved {
		return fmt.Errorf("交错巡检读回失败：%v，响应 %s", err, listed.text())
	}
	lastOrder := 0
	for index, record := range records {
		order := keeperOrder(record.Keeper)
		if order < 0 || order < lastOrder {
			return fmt.Errorf("巡检归属必须单调推进，第 %d 条为 %q：%s", index, record.Keeper, listed.text())
		}
		lastOrder = order
	}
	return nil
}
