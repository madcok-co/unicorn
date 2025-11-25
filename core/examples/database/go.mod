module github.com/madcok-co/unicorn/examples/database

go 1.24.0

require (
	github.com/madcok-co/unicorn v0.0.0
	gorm.io/driver/sqlite v1.6.0
	gorm.io/gorm v1.31.1
)

require (
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/mattn/go-sqlite3 v1.14.22 // indirect
	golang.org/x/text v0.31.0 // indirect
)

replace github.com/madcok-co/unicorn => ../../../
