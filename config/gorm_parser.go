package config

import (
	"fmt"
	"github.com/ninenhan/go-chat/model"
	"github.com/ninenhan/go-workflow/flow"
	"github.com/ninenhan/go-workflow/fn"
	"gorm.io/gorm"
	"strings"
)

func ApplyCondition(db *gorm.DB, cond flow.Condition) *gorm.DB {
	if cond.Key == "" || cond.Operator == "" {
		return db
	}
	key := fn.CamelToSnake(cond.Key)
	op := strings.ToUpper(strings.TrimSpace(cond.Operator))

	var (
		query any
		args  []any
	)

	switch op {
	case flow.EQ.Value:
		query, args = fmt.Sprintf("%s = ?", key), []any{cond.Value}
	case flow.NE.Value:
		query, args = fmt.Sprintf("%s <> ?", key), []any{cond.Value}
	case flow.GT.Value:
		query, args = fmt.Sprintf("%s > ?", key), []any{cond.Value}
	case flow.LT.Value:
		query, args = fmt.Sprintf("%s < ?", key), []any{cond.Value}
	case flow.GTE.Value:
		query, args = fmt.Sprintf("%s >= ?", key), []any{cond.Value}
	case flow.LTE.Value:
		query, args = fmt.Sprintf("%s <= ?", key), []any{cond.Value}
	case flow.LIKE.Value:
		query, args = fmt.Sprintf("%s LIKE ?", key), []any{fmt.Sprintf("%%%v%%", cond.Value)}
	case flow.IN.Value:
		query, args = fmt.Sprintf("%s IN ?", key), []any{cond.Value}
	case flow.NOT_IN.Value:
		// “值不在集合或为空”的语义： (col IS NULL OR col NOT IN (?))
		query, args = fmt.Sprintf("%s IS NULL OR %s NOT IN ?", key, key), []any{cond.Value}
	case flow.EXISTS.Value:
		query, args = fmt.Sprintf("%s IS NOT NULL AND %s <> ''", key, key), nil
	case flow.NON_EXISTS.Value:
		query, args = fmt.Sprintf("%s IS NULL OR %s = ''", key, key), nil
	default:
		// 未识别，直接返回，不影响链路
		return db
	}

	return db.Where(query, args...)
}

func ApplyConditionsWithConnector(db *gorm.DB, conditions []flow.Condition, connector flow.LogicConnector) *gorm.DB {
	if len(conditions) == 0 {
		return db
	}
	// 把单个 Condition（可能是叶子或分组）构造成“被括号包裹”的 *gorm.DB 片段
	buildPiece := func(c flow.Condition) *gorm.DB {
		tx := db.Session(&gorm.Session{NewDB: true})
		if len(c.Children) > 0 {
			//分组递归构造子组，子组内部用自己的 Connector
			return ApplyConditionsWithConnector(tx, c.Children, c.Connector)
		}
		// 叶子按操作符拼接
		return ApplyCondition(tx, c)
	}
	// 先构造首个分片
	acc := buildPiece(conditions[0])

	// 累加其余分片
	switch connector {
	case flow.AND, flow.OR:
		for i := 1; i < len(conditions); i++ {
			p := buildPiece(conditions[i])
			if connector == flow.AND {
				acc = acc.Where(p) // (A) AND (B)
			} else {
				acc = acc.Or(p) // (A) OR (B)
			}
		}
		// 整组挂到外层，并保持括号
		return db.Where(acc)
	case flow.NOT:
		// NOT 层：先用 AND 收敛，再整体取反
		for i := 1; i < len(conditions); i++ {
			p := buildPiece(conditions[i])
			acc = acc.Where(p) // AND 收敛
		}
		// NOT( acc )
		return db.Where("NOT (?)", acc)
	default:
		// 兜底按 AND 处理
		for i := 1; i < len(conditions); i++ {
			p := buildPiece(conditions[i])
			acc = acc.Where(p)
		}
		return db.Where(acc)
	}
}

func ApplyConditions(tx *gorm.DB, conditions []flow.Condition) *gorm.DB {
	return ApplyConditionsWithConnector(tx, conditions, flow.AND)
}

func ApplySorts(db *gorm.DB, sorter []model.SorterRO) *gorm.DB {
	for _, s := range sorter {
		direction := "ASC"
		desc := s.Desc
		if desc == nil {
			continue
		}
		direction = fn.Ternary(*desc, "DESC", "ASC")
		db = db.Order(fmt.Sprintf("%s %s", fn.CamelToSnake(s.Column), direction))
	}
	return db
}
