//
// Copyright (c) 2017 Intel Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

// Package tfortools provides a set of functions that are designed to
// make it easier for developers to add template based scripting to their
// command line tools.
//
// Command line tools written in Go often allow users to specify a template
// script to tailor the output of the tool to their specific needs. This can be
// useful both when visually inspecting the data and also when invoking command
// line tools in scripts. The best example of this is go list which allows users
// to pass a template script to extract interesting information about Go
// packages. For example,
//
//  go list -f '{{range .Imports}}{{println .}}{{end}}'
//
// prints all the imports of the current package.
//
// The aim of this package is to make it easier for developers to add template
// scripting support to their tools and easier for users of these tools to
// extract the information they need.   It does this by augmenting the
// templating language provided by the standard library package text/template in
// two ways:
//
// 1. It auto generates descriptions of the data structures passed as
// input to a template script for use in help messages.  This ensures
// that help usage information is always up to date with the source code.
//
// 2. It provides a suite of convenience functions to make it easy for
// script writers to extract the data they need.  There are functions for
// sorting, selecting rows and columns and generating nicely formatted
// tables.
//
// For example, if a program passed a slice of structs containing stock
// data to a template script, we could use the following script to extract
// the names of the 3 stocks with the highest trade volume.
//
//  {{table (cols (head (sort . "Volume" "dsc") 3) "Name" "Volume")}}
//
// The output might look something like this:
//
//  Name              Volume
//  Happy Enterprises 6395624278
//  Big Company       7500000
//  Medium Company    300122
//
// The functions head, sort, tables and col are provided by this package.
//
package tfortools

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/template"
	"unicode"
)

// BUG(markdryan): Map to slice

// These constants are used to ensure that all the help text
// for functions provided by this package are always presented
// in the same order.
const (
	helpFilterIndex = iota
	helpFilterContainsIndex
	helpFilterHasPrefixIndex
	helpFilterHasSuffixIndex
	helpFilterFoldedIndex
	helpFilterRegexpIndex
	helpToJSONIndex
	helpToCSVIndex
	helpSelectIndex
	helpSelectAltIndex
	helpTableIndex
	helpTableAltIndex
	helpTableXIndex
	helpTableXAltIndex
	helpHTableIndex
	helpHTableAltIndex
	helpHTableXIndex
	helpHTableXAltIndex
	helpColsIndex
	helpSortIndex
	helpRowsIndex
	helpHeadIndex
	helpTailIndex
	helpDescribeIndex
	helpPromoteIndex
	helpSliceofIndex
	helpToTableIndex
	helpIndexCount
)

type funcHelpInfo struct {
	name  string
	help  string
	index int
}

// Config is used to specify which functions should be added Go's template
// language.  It's not necessary to create a Config option.  Nil can be passed
// to all tfortools functions that take a Context object indicating the
// default behaviour is desired.  However, if you wish to restrict the
// number of functions added to Go's template language or you want to add your
// own functions, you'll need to create a Config object.  This can be done
// using the NewConfig function.
//
// All members of Config are private.
type Config struct {
	funcMap  template.FuncMap
	funcHelp []funcHelpInfo
}

func (c *Config) Len() int           { return len(c.funcHelp) }
func (c *Config) Swap(i, j int)      { c.funcHelp[i], c.funcHelp[j] = c.funcHelp[j], c.funcHelp[i] }
func (c *Config) Less(i, j int) bool { return c.funcHelp[i].index < c.funcHelp[j].index }

// AddCustomFn adds a custom function to the template language understood by
// tfortools.CreateTemplate and tfortools.OutputToTemplate.  The function
// implementation is provided by fn, its name, i.e., the name used to invoke the
// function in a program, is provided by name and the help for the function is
// provided by helpText.  An error will be returned if a function with the same
// name is already associated with this Config object.
func (c *Config) AddCustomFn(fn interface{}, name, helpText string) error {
	if _, found := c.funcMap[name]; found {
		return fmt.Errorf("%s already exists", name)
	}
	c.funcMap[name] = fn
	helpText = strings.TrimRightFunc(helpText, unicode.IsSpace)
	if helpText != "" {
		helpText = helpText + "\n"
		c.funcHelp = append(c.funcHelp,
			funcHelpInfo{name, helpText, helpIndexCount})
	}
	return nil
}

// OptAllFns enables all template extension functions provided by this package
func OptAllFns(c *Config) {
	c.funcMap = make(template.FuncMap)
	for k, v := range funcMap {
		c.funcMap[k] = v
	}

	c.funcHelp = make([]funcHelpInfo, len(funcHelpSlice), cap(funcHelpSlice))
	for i, h := range funcHelpSlice {
		c.funcHelp[i] = h
	}
}

const helpFilter = `- 'filter' operates on an slice or array of structures.  It allows the caller
  to filter the input array based on the value of a single field.
  The function returns a slice containing only the objects that satisfy the
  filter, e.g.

  {{len (filter . "Protected" "true")}}

  outputs the number of elements whose "Protected" field is equal to "true".
`

// OptFilter indicates that the filter function should be enabled.
// 'filter' operates on an slice or array of structures.  It allows the caller
// to filter the input array based on the value of a single field.
// The function returns a slice containing only the objects that satisfy the
// filter, e.g.
//
//  {{len (filter . "Protected" "true")}}
//
// outputs the number of elements whose "Protected" field is equal to "true".
func OptFilter(c *Config) {
	if _, ok := c.funcMap["filter"]; ok {
		return
	}
	c.funcMap["filter"] = filterByField
	c.funcHelp = append(c.funcHelp, funcHelpInfo{"filter", helpFilter, helpFilterIndex})
}

const helpFilterContains = `- 'filterContains' operates along the same lines as filter, but returns
  substring matches

  {{len(filterContains . "Name" "Cloud"}})

  outputs the number of elements whose "Name" field contains the word "Cloud".
`

// OptFilterContains indicates that the filterContains function should be
// enabled.  'filterContains' operates along the same lines as filter, but returns
// substring matches
//
//  {{len(filterContains . "Name" "Cloud"}})
//
// outputs the number of elements whose "Name" field contains the word "Cloud".
func OptFilterContains(c *Config) {
	if _, ok := c.funcMap["filterContains"]; ok {
		return
	}
	c.funcMap["filterContains"] = filterByContains
	c.funcHelp = append(c.funcHelp,
		funcHelpInfo{"filterContains", helpFilterContains, helpFilterContainsIndex})
}

const helpFilterHasPrefix = `- 'filterHasPrefix' is similar to filter, but returns prefix matches.
`

// OptFilterHasPrefix indicates that the filterHasPrefix function should be enabled.
// 'filterHasPrefix' is similar to filter, but returns prefix matches.
func OptFilterHasPrefix(c *Config) {
	if _, ok := c.funcMap["filterHasPrefix"]; ok {
		return
	}
	c.funcMap["filterHasPrefix"] = filterByHasPrefix
	c.funcHelp = append(c.funcHelp,
		funcHelpInfo{"filterHasPrefix", helpFilterHasPrefix, helpFilterHasPrefixIndex})
}

const helpFilterHasSuffix = `- 'filterHasSuffix' is similar to filter, but returns suffix matches.
`

// OptFilterHasSuffix indicates that the filterHasSuffix function should be enabled.
// 'filterHasSuffix' is similar to filter, but returns suffix matches.
func OptFilterHasSuffix(c *Config) {
	if _, ok := c.funcMap["filterHasSuffix"]; ok {
		return
	}
	c.funcMap["filterHasSuffix"] = filterByHasSuffix
	c.funcHelp = append(c.funcHelp,
		funcHelpInfo{"filterHasSuffix", helpFilterHasSuffix, helpFilterHasSuffixIndex})
}

const helpFilterFolded = `- 'filterFolded' is similar to filter, but returns matches based on equality
  under Unicode case-folding.
`

// OptFilterFolded indicates that the filterFolded function should be enabled.
// 'filterFolded' is similar to filter, but returns matches based on equality
// under Unicode case-folding.
func OptFilterFolded(c *Config) {
	if _, ok := c.funcMap["filterFolded"]; ok {
		return
	}
	c.funcMap["filterFolded"] = filterByFolded
	c.funcHelp = append(c.funcHelp,
		funcHelpInfo{"filterFolded", helpFilterFolded, helpFilterFoldedIndex})
}

const helpFilterRegexp = `- 'filterRegexp' is similar to filter, but returns matches based on regular
  expression matching

  {{len (filterRegexp . "Name" "^Docker[ a-zA-z]*latest$"}})

  outputs the number of elements whose "Name" field have 'Docker' as a prefix
  and 'latest' as a suffix in their name.
`

// OptFilterRegexp indicates that the filterRegexp function should be enabled.
// 'filterRegexp' is similar to filter, but returns matches based on regular
// expression matching
//
//  {{len (filterRegexp . "Name" "^Docker[ a-zA-z]*latest$"}})
//
// outputs the number of elements whose "Name" field have 'Docker' as a prefix
// and 'latest' as a suffix in their name.
func OptFilterRegexp(c *Config) {
	if _, ok := c.funcMap["filterRegexp"]; ok {
		return
	}
	c.funcMap["filterRegexp"] = filterByRegexp
	c.funcHelp = append(c.funcHelp,
		funcHelpInfo{"filterRegexp", helpFilterRegexp, helpFilterRegexpIndex})
}

// OptAllFilters is a convenience function that enables the following functions;
// 'filter', 'filterContains', 'filterHasPrefix', 'filterHasSuffix', 'filterFolded',
// and 'filterRegexp'
func OptAllFilters(c *Config) {
	OptFilter(c)
	OptFilterContains(c)
	OptFilterHasPrefix(c)
	OptFilterHasSuffix(c)
	OptFilterFolded(c)
	OptFilterRegexp(c)
}

const helpToJSON = `- 'tojson' outputs the target object in json format, e.g., {{tojson .}}
`

// OptToJSON indicates that the 'tosjon' function should be enabled.
// 'tojson' outputs the target object in json format, e.g., {{tojson .}}
func OptToJSON(c *Config) {
	if _, ok := c.funcMap["tojson"]; ok {
		return
	}
	c.funcMap["tojson"] = toJSON
	c.funcHelp = append(c.funcHelp, funcHelpInfo{"tojson", helpToJSON, helpToJSONIndex})
}

const helpToCSV = `- 'tocsv' converts a [][]string or a slice of structs to csv format, e.g.,
  {{tocsv .}}

  'tocsv' takes an optional boolean parameter, which if true, omits the
  first row containing the structure field name derived column headings.
  This boolean parameter defaults to false and is ignored when operating
  on a [][]string.
`

// OptToCSV indicates that the 'tocsv' function should be enabled.
// 'tocsv' converts a [][]string or a slice of structs to csv format, e.g.,
// {{tocsv .}}
//
// 'tocsv' takes an optional boolean parameter, which if true, omits the
// first row containing the structure field name derived column headings.
// This boolean parameter defaults to false and is ignored when operating
// on a [][]string.
func OptToCSV(c *Config) {
	if _, ok := c.funcMap["tocsv"]; ok {
		return
	}
	c.funcMap["tocsv"] = toCSV
	c.funcHelp = append(c.funcHelp, funcHelpInfo{"tocsv", helpToCSV, helpToCSVIndex})
}

const helpSelect = `- 'select' operates on a slice of structs.  It outputs the value of a specified
  field for each struct on a new line , e.g.,

  {{select . "Name"}}

  prints the 'Name' field of each structure in the slice.
`

// OptSelect indicates that the 'select' function should be enabled.
// 'select' operates on a slice of structs.  It outputs the value of a specified
// field for each struct on a new line , e.g.,
//
//  {{select . "Name"}}
//
// prints the 'Name' field of each structure in the slice.
func OptSelect(c *Config) {
	if _, ok := c.funcMap["select"]; ok {
		return
	}
	c.funcMap["select"] = selectField
	c.funcHelp = append(c.funcHelp, funcHelpInfo{"select", helpSelect, helpSelectIndex})
}

const helpSelectAlt = `- 'selectalt' Similar to select except that objects are formatted using %#v
`

// OptSelectAlt indicates that the 'selectalt' function should be enabled.
// 'selectalt' Similar to select except that objects are formatted using %#v
func OptSelectAlt(c *Config) {
	if _, ok := c.funcMap["selectalt"]; ok {
		return
	}
	c.funcMap["selectalt"] = selectFieldAlt
	c.funcHelp = append(c.funcHelp,
		funcHelpInfo{"selectalt", helpSelectAlt, helpSelectAltIndex})
}

const helpTable = `- 'table' outputs a table given an array or a slice of structs.  The table
  headings are taken from the names of the structs fields.  Hidden fields and
  fields of type channel are ignored.  The tabwidth and minimum column width
  are hardcoded to 8.  An example of table's usage is

  {{table .}}
`

// OptTable indicates that the 'table' function should be enabled.
// 'table' outputs a table given an array or a slice of structs.  The table
// headings are taken from the names of the structs fields.  Hidden fields and
// fields of type channel are ignored.  The tabwidth and minimum column width
// are hardcoded to 8.  An example of table's usage is
//
//  {{table .}}
func OptTable(c *Config) {
	if _, ok := c.funcMap["table"]; ok {
		return
	}
	c.funcMap["table"] = table
	c.funcHelp = append(c.funcHelp, funcHelpInfo{"table", helpTable, helpTableIndex})
}

const helpTableAlt = `- 'tablealt' Similar to table except that objects are formatted using %#v
`

// OptTableAlt indicates that the 'tablealt' function should be enabled.
// 'tablealt' Similar to table except that objects are formatted using %#v
func OptTableAlt(c *Config) {
	if _, ok := c.funcMap["tablealt"]; ok {
		return
	}
	c.funcMap["tablealt"] = tableAlt
	c.funcHelp = append(c.funcHelp,
		funcHelpInfo{"tablealt", helpTableAlt, helpTableAltIndex})
}

const helpTableX = `- 'tablex' is similar to table but it allows the caller more control over the
  table's appearance.  Users can control the names of the headings and also set
  the tab and column width.  'tablex' takes 4 or more parameters.  The first
  parameter is the slice of structs to output, the second is the minimum column
  width, the third the tab width  and the fourth is the padding.  The fifth and
  subsequent parameters are the names of the column headings.  The column
  headings are optional and the field names of the structure will be used if
  they are absent.  Example of its usage are:

  {{tablex . 12 8 1 "Column 1" "Column 2"}}
  {{tablex . 8 8 1}}
`

// OptTableX indicates that the 'tablex' function should be enabled. 'tablex' is
// similar to table but it allows the caller more control over the table's
// appearance. Users can control the names of the headings and also set the tab
// and column width. 'tablex' takes 4 or more parameters. The first parameter is
// the slice of structs to output, the second is the minimum column width, the
// third the tab width and the fourth is the padding. The fifth and subsequent
// parameters are the names of the column headings. The column headings are
// optional and the field names of the structure will be used if they are
// absent. Example of its usage are:
//
//  {{tablex . 12 8 1 "Column 1" "Column 2"}}
//  {{tablex . 8 8 1}}
func OptTableX(c *Config) {
	if _, ok := c.funcMap["tablex"]; ok {
		return
	}
	c.funcMap["tablex"] = tablex
	c.funcHelp = append(c.funcHelp, funcHelpInfo{"tablex", helpTableX, helpTableXIndex})
}

const helpTableXAlt = `- 'tablexalt' Similar to tablex except that objects are formatted using %#v
`

// OptTableXAlt indicates that the 'tablexalt' function should be enabled.
// 'tablexalt' Similar to tablex except that objects are formatted using %#v
func OptTableXAlt(c *Config) {
	if _, ok := c.funcMap["tablexalt"]; ok {
		return
	}
	c.funcMap["tablexalt"] = tablexAlt
	c.funcHelp = append(c.funcHelp,
		funcHelpInfo{"tablexalt", helpTableXAlt, helpTableXAltIndex})
}

const helpHTable = `- 'htable' outputs each element of an array or a slice of structs
  in its own two column table.  The values for the first column are taken from
  the names of the structs' fields.  The second column contains the field values.
  Hidden fields and fields of type channel are ignored. The tabwidth and minimum
  column width are hardcoded to 8.  An example of htable's usage is

  {{htable .}}
`

// OptHTable indicates that the 'htable' function should be enabled.
// 'htable' outputs each element of an array or a slice of structs in its own
// two column table.  The values for the first column are taken from
// the names of the structs' fields.  The second column contains the field values.
// Hidden fields and fields of type channel are ignored. The tabwidth and minimum
// column width are hardcoded to 8.  An example of htable's usage is
//
//  {{htable .}}
func OptHTable(c *Config) {
	if _, ok := c.funcMap["htable"]; ok {
		return
	}
	c.funcMap["htable"] = htable
	c.funcHelp = append(c.funcHelp, funcHelpInfo{"htable", helpHTable, helpHTableIndex})
}

const helpHTableAlt = `- 'htablealt' Similar to htable except that objects are formatted using %#v
`

// OptHTableAlt indicates that the 'htablealt' function should be enabled.
// 'htablealt' Similar to table except that objects are formatted using %#v
func OptHTableAlt(c *Config) {
	if _, ok := c.funcMap["htablealt"]; ok {
		return
	}
	c.funcMap["htablealt"] = htableAlt
	c.funcHelp = append(c.funcHelp,
		funcHelpInfo{"htablealt", helpHTableAlt, helpHTableAltIndex})
}

const helpHTableX = `- 'htablex' is similar to htable but it allows the caller more control over the
  tables' appearances.  Users can control the names displayed in the first column
  and also set the tab and column width.  'htablex' takes 4 or more parameters.
  The first parameter is the slice of structs to output, the second is the minimum
  column width, the third the tab width  and the fourth is the padding.  The
  fifth and subsequent parameters are the values displayed in the first column
  of each table.  These first column values are optional and the field names of
  the structures will be used if they are absent.  Example of its usage are:

  {{htablex . 12 8 1 "Field 1" "Field 2"}}
  {{htablex . 8 8 1}}
`

// OptHTableX indicates that the 'htablex' function should be enabled.  'htablex'
// is similar to htable but it allows the caller more control over the
// tables' appearances.  Users can control the names displayed in the first column
// and also set the tab and column width.  'htablex' takes 4 or more parameters.
// The first parameter is the slice of structs to output, the second is the minimum
// column width, the third the tab width  and the fourth is the padding.  The
// fifth and subsequent parameters are the values displayed in the first column of
// each table.  These first column values are optional and the field names of the
// structures will be used if they are absent.  Example of its usage are:
//
//  {{htablex . 12 8 1 "Field 1" "Field 2"}}
//  {{htablex . 8 8 1}}
func OptHTableX(c *Config) {
	if _, ok := c.funcMap["htablex"]; ok {
		return
	}
	c.funcMap["htablex"] = htablex
	c.funcHelp = append(c.funcHelp, funcHelpInfo{"htablex", helpHTableX, helpHTableXIndex})
}

const helpHTableXAlt = `- 'htablexalt' Similar to htablex except that objects are formatted using %#v
`

// OptHTableXAlt indicates that the 'htablexalt' function should be enabled.
// 'htablexalt' Similar to htablex except that objects are formatted using %#v
func OptHTableXAlt(c *Config) {
	if _, ok := c.funcMap["htablexalt"]; ok {
		return
	}
	c.funcMap["htablexalt"] = htablexAlt
	c.funcHelp = append(c.funcHelp,
		funcHelpInfo{"htablexalt", helpHTableXAlt, helpHTableXAltIndex})
}

const helpCols = `- 'cols' can be used to extract certain columns from a table consisting of a
  slice or array of structs.  It returns a new slice of structs which contain
  only the fields requested by the caller.   For example, given a slice of structs

  {{cols . "Name" "Address"}}

  returns a new slice of structs, each element of which is a structure with only
  two fields, 'Name' and 'Address'.
`

// OptCols indicates that the 'cols' function should be enabled.
// 'cols' can be used to extract certain columns from a table consisting of a
// slice or array of structs.  It returns a new slice of structs which contain
// only the fields requested by the caller.   For example, given a slice of structs
//
//  {{cols . "Name" "Address"}}
//
// returns a new slice of structs, each element of which is a structure with only
// two fields, 'Name' and 'Address'.
func OptCols(c *Config) {
	if _, ok := c.funcMap["cols"]; ok {
		return
	}
	c.funcMap["cols"] = cols
	c.funcHelp = append(c.funcHelp, funcHelpInfo{"cols", helpCols, helpColsIndex})
}

const helpSort = `- 'sort' sorts a slice or an array of structs.  It takes three parameters.  The
  first is the slice; the second is the name of the structure field by which to
  'sort'; the third provides the direction of the 'sort'.  The third parameter is
  optional.  If provided, it must be either "asc" or "dsc".  If omitted the
  elements of the slice are sorted in ascending order.  The type of the second
  field can be a number or a string.  When presented with another type, 'sort'
  will try to sort the elements by the string representation of the chosen field.
  The following example sorts a slice in ascending order by the Name field.
  
  {{sort . "Name"}}
`

// OptSort indicates that the 'sort' function should be enabled.
// 'sort' sorts a slice or an array of structs.  It takes three parameters.  The
// first is the slice; the second is the name of the structure field by which to
// 'sort'; the third provides the direction of the 'sort'.  The third parameter is
// optional.  If provided, it must be either "asc" or "dsc".  If omitted the
// elements of the slice are sorted in ascending order.  The type of the second
// field can be a number or a string.  When presented with another type, 'sort'
// will try to sort the elements by the string representation of the chosen field.
// The following example sorts a slice in ascending order by the Name field.
//
//  {{sort . "Name"}}
func OptSort(c *Config) {
	if _, ok := c.funcMap["sort"]; ok {
		return
	}
	c.funcMap["sort"] = sortSlice
	c.funcHelp = append(c.funcHelp, funcHelpInfo{"sort", helpSort, helpSortIndex})
}

const helpRows = `- 'rows' is used to extract a set of given rows from a slice or an array.  It
  takes at least two parameters. The first is the slice on which to operate.
  All subsequent parameters must be integers that correspond to a row in the
  input slice.  Indicies that refer to non-existent rows are ignored.  For
  example:

  {{rows . 1 2}}

  extracts the 2nd and 3rd rows from the slice represented by '.'.
`

// OptRows indicates that the 'rows' function should be enabled.
// 'rows' is used to extract a set of given rows from a slice or an array.  It
// takes at least two parameters. The first is the slice on which to operate.
// All subsequent parameters must be integers that correspond to a row in the
// input slice.  Indicies that refer to non-existent rows are ignored.  For
// example:
//
//  {{rows . 1 2}}
//
// extracts the 2nd and 3rd rows from the slice represented by '.'.
func OptRows(c *Config) {
	if _, ok := c.funcMap["rows"]; ok {
		return
	}
	c.funcMap["rows"] = rows
	c.funcHelp = append(c.funcHelp, funcHelpInfo{"rows", helpRows, helpRowsIndex})
}

const helpHead = `- 'head' operates on a slice or an array, returning the first n elements of
  that array as a new slice.  If n is not provided, a slice containing the
  first element of the input slice is returned.  For example,

  {{ head .}}

  returns a single element slice containing the first element of '.' and

  {{ head . 3}}

  returns a slice containing the first three elements of '.'.  If '.' contains
  only 2 elements the slice returned by {{ head . 3}} would be identical to the
  input slice.
`

// OptHead indicates that the 'head' function should be enabled.
// 'head' operates on a slice or an array, returning the first n elements of
// that array as a new slice.  If n is not provided, a slice containing the
// first element of the input slice is returned.  For example,
//
//  {{ head .}}
//
// returns a single element slice containing the first element of '.' and
//
//  {{ head . 3}}
//
// returns a slice containing the first three elements of '.'.  If '.' contains
// only 2 elements the slice returned by
//  {{ head . 3}}
// would be identical to the input slice.
func OptHead(c *Config) {
	if _, ok := c.funcMap["head"]; ok {
		return
	}
	c.funcMap["head"] = head
	c.funcHelp = append(c.funcHelp, funcHelpInfo{"head", helpHead, helpHeadIndex})
}

const helpTail = `- 'tail' is similar to head except that it returns a slice containing the last
  n elements of the input slice.  For example,

  {{tail . 2}}

  returns a new slice containing the last two elements of '.'.
`

// OptTail indicates that the 'tail' function should be enabled.
// 'tail' is similar to head except that it returns a slice containing the last
// n elements of the input slice.  For example,
//
//  {{tail . 2}}
//
// returns a new slice containing the last two elements of '.'.
func OptTail(c *Config) {
	if _, ok := c.funcMap["tail"]; ok {
		return
	}
	c.funcMap["tail"] = tail
	c.funcHelp = append(c.funcHelp, funcHelpInfo{"tail", helpTail, helpTailIndex})
}

const helpDescribe = `- 'describe' takes a single argument and outputs a description of the
  type of that argument.  It can be useful if the type of the object
  operated on by a template program is not described in the help of the
  tool that executes the template.

  {{describe . }}

 outputs a description of the type of '.'.
`

// OptDescribe indicates that the 'describe' function should be enabled.
// 'describe' takes a single argument and outputs a description of the
// type of that argument.  It can be useful if the type of the object
// operated on by a template program is not described in the help of the
// tool that executes the template.
//
//  {{describe . }}
//
// outputs a description of the type of '.'.
func OptDescribe(c *Config) {
	if _, ok := c.funcMap["describe"]; ok {
		return
	}
	c.funcMap["describe"] = describe
	c.funcHelp = append(c.funcHelp,
		funcHelpInfo{"describe", helpDescribe, helpDescribeIndex})
}

const helpPromote = `- 'promote' takes two arguments, a slice or an array of structures and a field
  path.  It returns a new slice containing only the objects identified by the
  field path.  The field path is a period separated list of structure field
  names.  Promote can be useful to extract a set of objects deep within a data
  structure into a new slice that can be passed to other functions, e.g., table.
  For example, given the following type
 
   []struct {
      uninteresting int
      user struct {
          credentials struct {
              name string
              password string
          }
      }
  }

  {{promote . "user.credentials"}}

  returns a slice of credentials one for each element of the original top level
  slice.
`

// OptPromote indicates that the 'promote' function should be enabled.
// 'promote' takes two arguments, a slice or an array of structures and a field
// path.  It returns a new slice containing only the objects identified by the
// field path.  The field path is a period separated list of structure field
// names.  Promote can be useful to extract a set of objects deep within a data
// structure into a new slice that can be passed to other functions, e.g., table.
// For example, given the following type
//
//  []struct {
//      uninteresting int
//      user struct {
//          credentials struct {
//              name string
//              password string
//          }
//      }
//  }
//
//  {{promote . "user.credentials"}}
//
// returns a slice of credentials one for each element of the original top level
// slice.
func OptPromote(c *Config) {
	if _, ok := c.funcMap["promote"]; ok {
		return
	}
	c.funcMap["promote"] = promote
	c.funcHelp = append(c.funcHelp, funcHelpInfo{"promote", helpPromote, helpPromoteIndex})
}

const helpSliceof = `- 'sliceof' takes one argument and returns a new slice containing only that
argument.
`

// OptSliceof indicates that the 'sliceof' function should be enabled.  'sliceof'
// takes one argument and returns a new slice containing only that argument.
func OptSliceof(c *Config) {
	if _, ok := c.funcMap["sliceof"]; ok {
		return
	}
	c.funcMap["sliceof"] = sliceof
	c.funcHelp = append(c.funcHelp, funcHelpInfo{"sliceof", helpSliceof, helpSliceofIndex})
}

const helpToTable = `- 'totable' converts a slice of a slice of strings into a slice of
  structures.  The field names of the structures are taken from the values of
  the first row in the slice.  The types of the fields are derived from the
  values specified in the second row.  The input slice should be of length 2
  or greater.  Data in columns in the second and subsequent rows should be
  homogenous.  The elements of the first row should be unique and ideally be
  valid exported variable names.  'totable' will try to sanitize the field
  names, if they are not valid go identifiers.
`

// OptToTable indicates that the 'totable' function should be enabled. 'totable'
// takes a slice of a slice of strings as an argument and returns a slice of
// structures.  The field names of the structures are taken from the values of
// the first row in the slice.  The types of the fields are derived from the
// values specified in the second row.  The input slice should be of length 2
// or greater.  Data in columns in the second and subsequent rows should be
// homogenous.  The elements of the first row should be unique and ideally
// be valid exported variable names.  'totable' will try to sanitize the field
// names, if they are not valid go identifiers.
func OptToTable(c *Config) {
	if _, ok := c.funcMap["totable"]; ok {
		return
	}
	c.funcMap["totable"] = toTable
	c.funcHelp = append(c.funcHelp, funcHelpInfo{"totable", helpToTable, helpToTableIndex})
}

// NewConfig creates a new Config object that can be passed to other functions
// in this package.  The Config option keeps track of which new functions are
// added to Go's template libray.  If this function is called without arguments,
// none of the functions defined in this package are enabled in the resulting Config
// object.  To control which functions get added specify some options, e.g.,
//
//  ctx := tfortools.NewConfig(tfortools.OptHead, tfortools.OptTail)
//
// creates a new Config object that enables the 'head' and 'tail' functions only.
//
// To add all the functions, use the OptAllFNs options, e.g.,
//
//    ctx := tfortools.NewConfig(tfortools.OptAllFNs)
func NewConfig(options ...func(*Config)) *Config {
	c := &Config{
		funcMap: make(template.FuncMap),
	}
	for _, f := range options {
		f(c)
	}
	sort.Sort(c)

	return c
}

// TemplateFunctionHelp generates formatted documentation that describes the
// additional functions that the Config object c adds to Go's templating language.
// If c is nil, documentation is generated for all functions provided by
// tfortools.
func TemplateFunctionHelp(c *Config) string {
	b := &bytes.Buffer{}
	_, _ = b.WriteString("Some new functions have been added to Go's template language\n\n")

	for _, h := range getHelpers(c) {
		_, _ = b.WriteString(h.help)
	}
	return b.String()
}

// TemplateFunctionNames returns a slice of all the functions enabled in the
// supplied Config object.
func TemplateFunctionNames(c *Config) []string {
	helpers := getHelpers(c)
	names := make([]string, len(helpers))
	for i := range helpers {
		names[i] = helpers[i].name
	}
	return names
}

// TemplateFunctionHelpSingle returns help for a single function specified by
// name.  An error is returned if the function cannot be found.
func TemplateFunctionHelpSingle(name string, c *Config) (string, error) {
	helpers := getHelpers(c)
	for i := range helpers {
		if helpers[i].name == name {
			return helpers[i].help, nil
		}
	}
	return "", fmt.Errorf("%s is not defined", name)
}

// OutputToTemplate executes the template, whose source is contained within the
// tmplSrc parameter, on the object obj.  The name of the template is given by
// the name parameter.  The results of the execution are output to w.
// The functions enabled in the cfg parameter will be made available to the
// template source code specified in tmplSrc.  If cfg is nil, all the
// additional functions provided by tfortools will be enabled.
func OutputToTemplate(w io.Writer, name, tmplSrc string, obj interface{}, cfg *Config) (err error) {
	t, err := template.New(name).Funcs(getFuncMap(cfg)).Parse(tmplSrc)
	if err != nil {
		return err
	}
	if err = t.Execute(w, obj); err != nil {
		return err
	}
	return nil
}

// CreateTemplate creates a new template, whose source is contained within the
// tmplSrc parameter and whose name is given by the name parameter. The functions
// enabled in the cfg parameter will be made available to the template source code
// specified in tmplSrc.  If cfg is nil, all the additional functions provided by
// tfortools will be enabled.
func CreateTemplate(name, tmplSrc string, cfg *Config) (*template.Template, error) {
	if tmplSrc == "" {
		return nil, fmt.Errorf("template %s contains no source", name)
	}

	return template.New(name).Funcs(getFuncMap(cfg)).Parse(tmplSrc)
}

// GenerateUsageUndecorated returns a formatted string identifying the
// elements of the type of object i that can be accessed from inside a template.
// Unexported struct values and channels are not output as they cannot be usefully
// accessed inside a template.
//
// The output produced by GenerateUsageUndecorated preserves structure tags.
// There is one special case however.  Tags with a key of "tfortools" are
// output as comments at the end of the line containing the field, rather
// than as tags.  This tag can be used to document your structures.
func GenerateUsageUndecorated(i interface{}) string {
	var buf bytes.Buffer
	generateIndentedUsage(&buf, i)
	return buf.String()
}

// GenerateUsageDecorated is similar to GenerateUsageUndecorated with the
// exception that it outputs the usage information for all the new functions
// enabled in the Config object cfg.  If cfg is nil, help information is
// printed for all new template functions defined by this package.
func GenerateUsageDecorated(flag string, i interface{}, cfg *Config) string {
	var buf bytes.Buffer

	fmt.Fprintf(&buf,
		"The template passed to the -%s option operates on a\n\n",
		flag)

	generateIndentedUsage(&buf, i)
	fmt.Fprintln(&buf)
	fmt.Fprintf(&buf, TemplateFunctionHelp(cfg))
	return buf.String()
}
