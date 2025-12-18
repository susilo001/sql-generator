package model

type SortDirection string
type Condition string
type Operator string

const (
	IsEqual           Operator = "IS_EQUAL"
	IsNotEqual        Operator = "IS_NOT_EQUAL"
	IsLessThan        Operator = "IS_LESS_THAN"
	IsMoreThan        Operator = "IS_MORE_THAN"
	IsLessThanOrEqual Operator = "IS_LESS_THAN_OR_EQUAL"
	IsMoreThanOrEqual Operator = "IS_MORE_THAN_OR_EQUAL"
	IsContain         Operator = "IS_CONTAIN"
	IsBeginWith       Operator = "IS_BEGIN_WITH"
	IsEndWith         Operator = "IS_END_WITH"
	IsBetween         Operator = "IS_BETWEEN"
	IsIn              Operator = "IS_IN"
	IsNotIn           Operator = "IS_NOT_IN"
	IsNull            Operator = "IS_NULL"
	IsNotNull         Operator = "IS_NOT_NULL"
)

const (
	Ascending  SortDirection = "ASCENDING"
	Descending SortDirection = "DESCENDING"
)

const (
	And Condition = "AND"
	Or  Condition = "OR"
)
