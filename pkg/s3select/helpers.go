/*
 * Minio Cloud Storage, (C) 2018 Minio, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package s3select

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/pydio/minio-srv/pkg/s3select/format"
	"github.com/tidwall/gjson"
	"github.com/xwb1989/sqlparser"
)

// MaxExpressionLength - 256KiB
const MaxExpressionLength = 256 * 1024

// matchesMyWhereClause takes map[string]interfaces{} , process the where clause and returns true if the row suffices
func matchesMyWhereClause(record map[string]interface{}, alias string, whereClause interface{}) (bool, error) {
	var conversionColumn string
	var operator string
	var operand interface{}
	if fmt.Sprintf("%v", whereClause) == "false" {
		return false, nil
	}
	out, err := json.Marshal(record)
	if err != nil {
		return false, ErrExternalEvalException
	}
	switch expr := whereClause.(type) {
	case *sqlparser.IsExpr:
		return evaluateIsExpr(expr, string(out), alias)
	case *sqlparser.RangeCond:
		operator = expr.Operator
		if operator != "between" && operator != "not between" {
			return false, ErrUnsupportedSQLOperation
		}
		if operator == "not between" {
			result, err := evaluateBetween(expr, alias, string(out))
			if err != nil {
				return false, err
			}
			return !result, nil
		}
		result, err := evaluateBetween(expr, alias, string(out))
		if err != nil {
			return false, err
		}
		return result, nil
	case *sqlparser.ComparisonExpr:
		operator = expr.Operator
		switch right := expr.Right.(type) {
		case *sqlparser.FuncExpr:
			operand = evaluateFuncExpr(right, "", string(out))
		case *sqlparser.SQLVal:
			var err error
			operand, err = evaluateParserType(right)
			if err != nil {
				return false, err
			}
		}
		var myVal string
		myVal = ""
		switch left := expr.Left.(type) {
		case *sqlparser.FuncExpr:
			myVal = evaluateFuncExpr(left, "", string(out))
			conversionColumn = ""
		case *sqlparser.ColName:
			conversionColumn = left.Name.CompliantName()
		}

		if myVal != "" {
			return evaluateOperator(myVal, operator, operand)
		}
		return evaluateOperator(jsonValue(conversionColumn, string(out)), operator, operand)
	case *sqlparser.AndExpr:
		var leftVal bool
		var rightVal bool
		switch left := expr.Left.(type) {
		case *sqlparser.ComparisonExpr:
			temp, err := matchesMyWhereClause(record, alias, left)
			if err != nil {
				return false, err
			}
			leftVal = temp
		}
		switch right := expr.Right.(type) {
		case *sqlparser.ComparisonExpr:
			temp, err := matchesMyWhereClause(record, alias, right)
			if err != nil {
				return false, err
			}
			rightVal = temp
		}
		return (rightVal && leftVal), nil
	case *sqlparser.OrExpr:
		var leftVal bool
		var rightVal bool
		switch left := expr.Left.(type) {
		case *sqlparser.ComparisonExpr:
			leftVal, _ = matchesMyWhereClause(record, alias, left)

		}
		switch right := expr.Right.(type) {
		case *sqlparser.ComparisonExpr:
			rightVal, _ = matchesMyWhereClause(record, alias, right)
		}
		return (rightVal || leftVal), nil
	}
	return true, nil
}

func applyStrFunc(rawArg string, funcName string) string {
	switch strings.ToUpper(funcName) {
	case "TRIM":
		// parser has an issue which does not allow it to support Trim with other
		// arguments
		return strings.Trim(rawArg, " ")
	case "SUBSTRING":
		// TODO parser has an issue which does not support substring
		return rawArg
	case "CHAR_LENGTH":
		return strconv.Itoa(len(rawArg))
	case "CHARACTER_LENGTH":
		return strconv.Itoa(len(rawArg))
	case "LOWER":
		return strings.ToLower(rawArg)
	case "UPPER":
		return strings.ToUpper(rawArg)
	}
	return rawArg

}

// evaluateBetween is a function which evaluates a Between Clause.
func evaluateBetween(betweenExpr *sqlparser.RangeCond, alias string, record string) (bool, error) {
	var colToVal interface{}
	var colFromVal interface{}
	var conversionColumn string
	var funcName string
	switch colTo := betweenExpr.To.(type) {
	case sqlparser.Expr:
		switch colToMyVal := colTo.(type) {
		case *sqlparser.FuncExpr:
			colToVal = stringOps(colToMyVal, record, "")
		case *sqlparser.SQLVal:
			var err error
			colToVal, err = evaluateParserType(colToMyVal)
			if err != nil {
				return false, err
			}
		}
	}
	switch colFrom := betweenExpr.From.(type) {
	case sqlparser.Expr:
		switch colFromMyVal := colFrom.(type) {
		case *sqlparser.FuncExpr:
			colFromVal = stringOps(colFromMyVal, record, "")
		case *sqlparser.SQLVal:
			var err error
			colFromVal, err = evaluateParserType(colFromMyVal)
			if err != nil {
				return false, err
			}
		}
	}
	var myFuncVal string
	switch left := betweenExpr.Left.(type) {
	case *sqlparser.FuncExpr:
		myFuncVal = evaluateFuncExpr(left, "", record)
		conversionColumn = ""
	case *sqlparser.ColName:
		conversionColumn = cleanCol(left.Name.CompliantName(), alias)
	}
	toGreater, err := evaluateOperator(fmt.Sprintf("%v", colToVal), ">", colFromVal)
	if err != nil {
		return false, err
	}
	if toGreater {
		return evalBetweenGreater(conversionColumn, record, funcName, colFromVal, colToVal, myFuncVal)
	}
	return evalBetweenLess(conversionColumn, record, funcName, colFromVal, colToVal, myFuncVal)
}

// evalBetweenGreater is a function which evaluates the between given that the
// TO is > than the FROM.
func evalBetweenGreater(conversionColumn string, record string, funcName string, colFromVal interface{}, colToVal interface{}, myColVal string) (bool, error) {
	if format.IsInt(conversionColumn) {
		myVal, err := evaluateOperator(jsonValue("_"+conversionColumn, record), ">=", colFromVal)
		if err != nil {
			return false, err
		}
		var myOtherVal bool
		myOtherVal, err = evaluateOperator(fmt.Sprintf("%v", colToVal), ">=", checkStringType(jsonValue("_"+conversionColumn, record)))
		if err != nil {
			return false, err
		}
		return (myVal && myOtherVal), nil
	}
	if myColVal != "" {
		myVal, err := evaluateOperator(myColVal, ">=", colFromVal)
		if err != nil {
			return false, err
		}
		var myOtherVal bool
		myOtherVal, err = evaluateOperator(fmt.Sprintf("%v", colToVal), ">=", checkStringType(myColVal))
		if err != nil {
			return false, err
		}
		return (myVal && myOtherVal), nil
	}
	myVal, err := evaluateOperator(jsonValue(conversionColumn, record), ">=", colFromVal)
	if err != nil {
		return false, err
	}
	var myOtherVal bool
	myOtherVal, err = evaluateOperator(fmt.Sprintf("%v", colToVal), ">=", checkStringType(jsonValue(conversionColumn, record)))
	if err != nil {
		return false, err
	}
	return (myVal && myOtherVal), nil
}

// evalBetweenLess is a function which evaluates the between given that the
// FROM is > than the TO.
func evalBetweenLess(conversionColumn string, record string, funcName string, colFromVal interface{}, colToVal interface{}, myColVal string) (bool, error) {
	if format.IsInt(conversionColumn) {
		// Subtract 1 out because the index starts at 1 for Amazon instead of 0.
		myVal, err := evaluateOperator(jsonValue("_"+conversionColumn, record), "<=", colFromVal)
		if err != nil {
			return false, err
		}
		var myOtherVal bool
		myOtherVal, err = evaluateOperator(fmt.Sprintf("%v", colToVal), "<=", checkStringType(jsonValue("_"+conversionColumn, record)))
		if err != nil {
			return false, err
		}
		return (myVal && myOtherVal), nil
	}
	if myColVal != "" {
		myVal, err := evaluateOperator(myColVal, "<=", colFromVal)
		if err != nil {
			return false, err
		}
		var myOtherVal bool
		myOtherVal, err = evaluateOperator(fmt.Sprintf("%v", colToVal), "<=", checkStringType(myColVal))
		if err != nil {
			return false, err
		}
		return (myVal && myOtherVal), nil
	}
	myVal, err := evaluateOperator(jsonValue(conversionColumn, record), "<=", colFromVal)
	if err != nil {
		return false, err
	}
	var myOtherVal bool
	myOtherVal, err = evaluateOperator(fmt.Sprintf("%v", colToVal), "<=", checkStringType(jsonValue(conversionColumn, record)))
	if err != nil {
		return false, err
	}
	return (myVal && myOtherVal), nil
}

// This is a really important function it actually evaluates the boolean
// statement and therefore actually returns a bool, it functions as the lowest
// level of the state machine.
func evaluateOperator(myTblVal string, operator string, operand interface{}) (bool, error) {
	if err := checkValidOperator(operator); err != nil {
		return false, err
	}
	myRecordVal := checkStringType(myTblVal)
	myVal := reflect.ValueOf(myRecordVal)
	myOp := reflect.ValueOf(operand)

	switch {
	case myVal.Kind() == reflect.String && myOp.Kind() == reflect.String:
		return stringEval(myVal.String(), operator, myOp.String())
	case myVal.Kind() == reflect.Float64 && myOp.Kind() == reflect.Float64:
		return floatEval(myVal.Float(), operator, myOp.Float())
	case myVal.Kind() == reflect.Int && myOp.Kind() == reflect.Int:
		return intEval(myVal.Int(), operator, myOp.Int())
	case myVal.Kind() == reflect.Int && myOp.Kind() == reflect.String:
		stringVs := strconv.Itoa(int(myVal.Int()))
		return stringEval(stringVs, operator, myOp.String())
	case myVal.Kind() == reflect.Float64 && myOp.Kind() == reflect.String:
		stringVs := strconv.FormatFloat(myVal.Float(), 'f', 6, 64)
		return stringEval(stringVs, operator, myOp.String())
	case myVal.Kind() != myOp.Kind():
		return false, nil
	}
	return false, ErrUnsupportedSyntax
}

// checkValidOperator ensures that the current operator is supported
func checkValidOperator(operator string) error {
	listOfOps := []string{">", "<", "=", "<=", ">=", "!=", "like"}
	for i := range listOfOps {
		if operator == listOfOps[i] {
			return nil
		}
	}
	return ErrParseUnknownOperator
}

// checkStringType converts the value from the csv to the appropriate one.
func checkStringType(tblVal string) interface{} {
	intVal, err := strconv.Atoi(tblVal)
	if err == nil {
		return intVal
	}
	floatVal, err := strconv.ParseFloat(tblVal, 64)
	if err == nil {
		return floatVal
	}
	return tblVal
}

// stringEval is for evaluating the state of string comparison.
func stringEval(myRecordVal string, operator string, myOperand string) (bool, error) {
	switch operator {
	case ">":
		return myRecordVal > myOperand, nil
	case "<":
		return myRecordVal < myOperand, nil
	case "=":
		return myRecordVal == myOperand, nil
	case "<=":
		return myRecordVal <= myOperand, nil
	case ">=":
		return myRecordVal >= myOperand, nil
	case "!=":
		return myRecordVal != myOperand, nil
	case "like":
		return likeConvert(myOperand, myRecordVal)
	}
	return false, ErrUnsupportedSyntax
}

// intEval is for evaluating integer comparisons.
func intEval(myRecordVal int64, operator string, myOperand int64) (bool, error) {

	switch operator {
	case ">":
		return myRecordVal > myOperand, nil
	case "<":
		return myRecordVal < myOperand, nil
	case "=":
		return myRecordVal == myOperand, nil
	case "<=":
		return myRecordVal <= myOperand, nil
	case ">=":
		return myRecordVal >= myOperand, nil
	case "!=":
		return myRecordVal != myOperand, nil
	}
	return false, ErrUnsupportedSyntax
}

// floatEval is for evaluating the comparison of floats.
func floatEval(myRecordVal float64, operator string, myOperand float64) (bool, error) {
	// Basically need some logic thats like, if the types dont match check for a cast
	switch operator {
	case ">":
		return myRecordVal > myOperand, nil
	case "<":
		return myRecordVal < myOperand, nil
	case "=":
		return myRecordVal == myOperand, nil
	case "<=":
		return myRecordVal <= myOperand, nil
	case ">=":
		return myRecordVal >= myOperand, nil
	case "!=":
		return myRecordVal != myOperand, nil
	}
	return false, ErrUnsupportedSyntax
}

// prefixMatch allows for matching a prefix only like query e.g a%
func prefixMatch(pattern string, record string) bool {
	for i := 0; i < len(pattern)-1; i++ {
		if pattern[i] != record[i] && pattern[i] != byte('_') {
			return false
		}
	}
	return true
}

// suffixMatch allows for matching a suffix only like query e.g %an
func suffixMatch(pattern string, record string) bool {
	for i := len(pattern) - 1; i > 0; i-- {
		if pattern[i] != record[len(record)-(len(pattern)-i)] && pattern[i] != byte('_') {
			return false
		}
	}
	return true
}

// This function is for evaluating select statements which are case sensitive
func likeConvert(pattern string, record string) (bool, error) {
	// If pattern is empty just return false
	if pattern == "" || record == "" {
		return false, nil
	}
	// for suffix match queries e.g %a
	if len(pattern) >= 2 && pattern[0] == byte('%') && strings.Count(pattern, "%") == 1 {
		return suffixMatch(pattern, record), nil
	}
	// for prefix match queries e.g a%
	if len(pattern) >= 2 && pattern[len(pattern)-1] == byte('%') && strings.Count(pattern, "%") == 1 {
		return prefixMatch(pattern, record), nil
	}
	charCount := 0
	currPos := 0
	// Loop through the pattern so that a boolean can be returned
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == byte('_') {
			// if its an underscore it can be anything so shift current position for
			// pattern and string
			charCount++
			// if there have been more characters in the pattern than record, clearly
			// there should be a return
			if i != len(pattern)-1 {
				if pattern[i+1] != byte('%') && pattern[i+1] != byte('_') {
					if currPos != len(record)-1 && pattern[i+1] != record[currPos+1] {
						return false, nil
					}
				}
			}
			if charCount > len(record) {
				return false, nil
			}
			// if the pattern has been fully evaluated, then just return.
			if len(pattern) == i+1 {
				return true, nil
			}
			i++
			currPos++
		}
		if pattern[i] == byte('%') || pattern[i] == byte('*') {
			// if there is a wildcard then want to return true if its last and flag it.
			if currPos == len(record) {
				return false, nil
			}
			if i+1 == len(pattern) {
				return true, nil
			}
		} else {
			charCount++
			matched := false
			// iterate through the pattern and check if there is a match for the
			// character
			for currPos < len(record) {
				if record[currPos] == pattern[i] || pattern[i] == byte('_') {
					matched = true
					break
				}
				currPos++
			}
			currPos++
			// if the character did not match then return should occur.
			if !matched {
				return false, nil
			}
		}
	}
	if charCount > len(record) {
		return false, nil
	}
	if currPos < len(record) {
		return false, nil
	}
	return true, nil
}

// cleanCol cleans a column name from the parser so that the name is returned to
// original.
func cleanCol(myCol string, alias string) string {
	if len(myCol) <= 0 {
		return myCol
	}
	if !strings.HasPrefix(myCol, alias) && myCol[0] == '_' {
		myCol = alias + myCol
	}

	if strings.Contains(myCol, ".") {
		myCol = strings.Replace(myCol, alias+"._", "", len(myCol))
	}
	myCol = strings.Replace(myCol, alias+"_", "", len(myCol))
	return myCol
}

// whereClauseNameErrs is a function which returns an error if there is a column
// in the where clause which does not exist.
func whereClauseNameErrs(whereClause interface{}, alias string, f format.Select) error {
	var conversionColumn string
	switch expr := whereClause.(type) {
	// case for checking errors within a clause of the form "col_name is ..."
	case *sqlparser.IsExpr:
		switch myCol := expr.Expr.(type) {
		case *sqlparser.FuncExpr:
			if err := evaluateFuncErr(myCol, f); err != nil {
				return err
			}
		case *sqlparser.ColName:
			conversionColumn = cleanCol(myCol.Name.CompliantName(), alias)
		}
	case *sqlparser.RangeCond:
		switch left := expr.Left.(type) {
		case *sqlparser.FuncExpr:
			if err := evaluateFuncErr(left, f); err != nil {
				return err
			}
		case *sqlparser.ColName:
			conversionColumn = cleanCol(left.Name.CompliantName(), alias)
		}
	case *sqlparser.ComparisonExpr:
		switch left := expr.Left.(type) {
		case *sqlparser.FuncExpr:
			if err := evaluateFuncErr(left, f); err != nil {
				return err
			}
		case *sqlparser.ColName:
			conversionColumn = cleanCol(left.Name.CompliantName(), alias)
		}
	case *sqlparser.AndExpr:
		switch left := expr.Left.(type) {
		case *sqlparser.ComparisonExpr:
			return whereClauseNameErrs(left, alias, f)
		}
		switch right := expr.Right.(type) {
		case *sqlparser.ComparisonExpr:
			return whereClauseNameErrs(right, alias, f)
		}
	case *sqlparser.OrExpr:
		switch left := expr.Left.(type) {
		case *sqlparser.ComparisonExpr:
			return whereClauseNameErrs(left, alias, f)
		}
		switch right := expr.Right.(type) {
		case *sqlparser.ComparisonExpr:
			return whereClauseNameErrs(right, alias, f)
		}
	}
	if conversionColumn != "" {
		return f.ColNameErrs([]string{conversionColumn})
	}
	return nil
}

// aggFuncToStr converts an array of floats into a properly formatted string.
func aggFuncToStr(aggVals []float64, f format.Select) string {
	// Define a number formatting function
	numToStr := func(f float64) string {
		if f == math.Trunc(f) {
			return strconv.FormatInt(int64(f), 10)
		}
		return strconv.FormatFloat(f, 'f', 6, 64)
	}

	// Display all whole numbers in aggVals as integers
	vals := make([]string, len(aggVals))
	for i, v := range aggVals {
		vals[i] = numToStr(v)
	}

	// Intersperse field delimiter
	return strings.Join(vals, f.OutputFieldDelimiter())
}

// checkForDuplicates ensures we do not have an ambigious column name.
func checkForDuplicates(columns []string, columnsMap map[string]int, hasDuplicates map[string]bool, lowercaseColumnsMap map[string]int) error {
	for i := 0; i < len(columns); i++ {
		columns[i] = strings.Replace(columns[i], " ", "_", len(columns[i]))
		if _, exist := columnsMap[columns[i]]; exist {
			return ErrAmbiguousFieldName
		}
		columnsMap[columns[i]] = i
		// This checks that if a key has already been put into the map, that we're
		// setting its appropriate value in has duplicates to be true.
		if _, exist := lowercaseColumnsMap[strings.ToLower(columns[i])]; exist {
			hasDuplicates[strings.ToLower(columns[i])] = true
		} else {
			lowercaseColumnsMap[strings.ToLower(columns[i])] = i
		}
	}
	return nil
}

// evaluateParserType is a function that takes a SQL value and returns it as an
// interface converted into the appropriate value.
func evaluateParserType(col *sqlparser.SQLVal) (interface{}, error) {
	colDataType := col.Type
	var val interface{}
	switch colDataType {
	case 0:
		val = string(col.Val)
	case 1:
		intVersion, isInt := strconv.Atoi(string(col.Val))
		if isInt != nil {
			return nil, ErrIntegerOverflow
		}
		val = intVersion
	case 2:
		floatVersion, isFloat := strconv.ParseFloat(string(col.Val), 64)
		if isFloat != nil {
			return nil, ErrIntegerOverflow
		}
		val = floatVersion
	}
	return val, nil
}

// parseErrs is the function which handles all the errors that could occur
// through use of function arguments such as column names in NULLIF
func parseErrs(columnNames []string, whereClause interface{}, alias string, myFuncs SelectFuncs, f format.Select) error {
	// Below code cleans up column names.
	processColumnNames(columnNames, alias, f)
	if columnNames[0] != "*" {
		if err := f.ColNameErrs(columnNames); err != nil {
			return err
		}
	}
	// Below code ensures the whereClause has no errors.
	if whereClause != nil {
		tempClause := whereClause
		if err := whereClauseNameErrs(tempClause, alias, f); err != nil {
			return err
		}
	}
	for i := 0; i < len(myFuncs.funcExpr); i++ {
		if myFuncs.funcExpr[i] == nil {
			continue
		}
		if err := evaluateFuncErr(myFuncs.funcExpr[i], f); err != nil {
			return err
		}
	}
	return nil
}

// It return the value corresponding to the tag in Json .
// Input is the Key and row is the JSON string
func jsonValue(input string, row string) string {
	value := gjson.Get(row, input)
	return value.String()
}
