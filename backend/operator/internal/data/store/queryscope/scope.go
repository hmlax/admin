// Package queryscope defines native GORM scopes used by generated model predicates.
package queryscope

import (
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Scope = func(*gorm.DB) *gorm.DB

func Equal(field string, value any) Scope {
	return Where(clause.Eq{
		Column: clause.Column{Name: field},
		Value:  value,
	})
}
func NotEqual(field string, value any) Scope {
	return Where(clause.Neq{
		Column: clause.Column{Name: field},
		Value:  value,
	})
}
func Greater(field string, value any) Scope {
	return Where(clause.Gt{
		Column: clause.Column{Name: field},
		Value:  value,
	})
}
func GreaterEqual(field string, value any) Scope {
	return Where(clause.Gte{
		Column: clause.Column{Name: field},
		Value:  value,
	})
}
func Less(field string, value any) Scope {
	return Where(clause.Lt{
		Column: clause.Column{Name: field},
		Value:  value,
	})
}
func LessEqual(field string, value any) Scope {
	return Where(clause.Lte{
		Column: clause.Column{Name: field},
		Value:  value,
	})
}
func In(field string, values ...any) Scope {
	return Where(clause.IN{
		Column: clause.Column{Name: field},
		Values: values,
	})
}
func NotIn(field string, values ...any) Scope { return Not(In(field, values...)) }
func IsNull(field string) Scope               { return Equal(field, nil) }
func NotNull(field string) Scope              { return NotEqual(field, nil) }
func Where(expression clause.Expression) Scope {
	return func(db *gorm.DB) *gorm.DB { return db.Where(expression) }
}

func escapeLike(value string) string {
	return strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(value)
}
func like(field, pattern string, fold bool) Scope {
	if fold {
		pattern = strings.ToLower(pattern)
	}
	return func(db *gorm.DB) *gorm.DB {
		// SQLite requires an explicit escape clause; MySQL/PostgreSQL accept it too.
		expression := "? LIKE ? ESCAPE '\\'"
		if db.Name() == "mysql" {
			expression = "? LIKE ? ESCAPE '\\\\'"
		}
		if fold {
			expression = "LOWER(" + expression
			expression = strings.Replace(expression, " LIKE", ") LIKE", 1)
		}
		return db.Where(clause.Expr{
			SQL:  expression,
			Vars: []any{clause.Column{Name: field}, pattern},
		})
	}
}
func Contains(field, value string) Scope     { return like(field, "%"+escapeLike(value)+"%", false) }
func ContainsFold(field, value string) Scope { return like(field, "%"+escapeLike(value)+"%", true) }
func HasPrefix(field, value string) Scope    { return like(field, escapeLike(value)+"%", false) }
func HasSuffix(field, value string) Scope    { return like(field, "%"+escapeLike(value), false) }
func EqualFold(field, value string) Scope    { return like(field, escapeLike(value), true) }

func group(scopes []Scope) Scope {
	return func(db *gorm.DB) *gorm.DB {
		child := db.Session(&gorm.Session{NewDB: true})
		for _, scope := range scopes {
			child = scope(child)
		}
		return child
	}
}
func And(scopes ...Scope) Scope {
	return func(db *gorm.DB) *gorm.DB {
		child := group(scopes)(db)
		if child.Error != nil {
			_ = db.AddError(child.Error)
			return db
		}
		return db.Where(child)
	}
}
func Or(scopes ...Scope) Scope {
	return func(db *gorm.DB) *gorm.DB {
		if len(scopes) == 0 {
			return db.Where("1 = 0")
		}
		child := db.Session(&gorm.Session{NewDB: true})
		for _, scope := range scopes {
			branch := group([]Scope{scope})(db)
			if branch.Error != nil {
				_ = db.AddError(branch.Error)
				return db
			}
			child = child.Or(branch)
		}
		return db.Where(child)
	}
}
func Not(scope Scope) Scope {
	return func(db *gorm.DB) *gorm.DB {
		child := group([]Scope{scope})(db)
		if child.Error != nil {
			_ = db.AddError(child.Error)
			return db
		}
		return db.Not(child)
	}
}

// OrderBy builds a quoted column order; input is never treated as SQL text.
func OrderBy(field string, descending bool) Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db.Order(clause.OrderByColumn{
			Column: clause.Column{Name: field},
			Desc:   descending,
		})
	}
}
