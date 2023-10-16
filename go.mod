module httptest
// main.go 和 geecache/ 在同级目录，但 go modules 不再支持 import <相对路径>，相对路径需要在 go.mod 中声明
require geecache v0.0.0
replace geecache => ./geecache
go 1.19
