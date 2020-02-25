goconfig [![Go Walker](http://gowalker.org/api/v1/badge)](http://gowalker.org/github.com/Unknwon/goconfig)
========

[中文文档](README_ZH.md)

**IMPORTANT** 

- This library is under bug fix only mode, which means no more features will be added.
- I'm continuing working on better Go code with a different library: [ini](https://github.com/go-ini/ini).

## About

Package goconfig is a easy-use, comments-support configuration file parser for the Go Programming Language, which provides a structure similar to what you would find on Microsoft Windows INI files.

The configuration file consists of sections, led by a `[section]` header and followed by `name:value` or `name=value` entries. Note that leading whitespace is removed from values. The optional values can contain format strings which refer to other values in the same section, or values in a special DEFAULT section. Comments are indicated by ";" or "#"; comments may begin anywhere on a single line.
	
## Features
	
- It simplified operation processes, easy to use and undersatnd; therefore, there are less chances to have errors. 
- It uses exactly the same way to access a configuration file as you use Windows APIs, so you don't need to change your code style.
- It supports read recursion sections.
- It supports auto increment of key.
- It supports **READ** and **WRITE** configuration file with comments each section or key which all the other parsers don't support!!!!!!!
- It supports get value through type bool, float64, int, int64 and string, methods that start with "Must" means ignore errors and get zero-value if error occurs, or you can specify a default value.
- It's able to load multiple files to overwrite key values. 

## Installation
	
	go get github.com/unknwon/goconfig

## API Documentation

[Go Walker](http://gowalker.org/github.com/unknwon/goconfig).

## Example

Please see [conf.ini](testdata/conf.ini) as an example.

### Usage

- Function `LoadConfigFile` load file(s) depends on your situation, and return a variable with type `ConfigFile`.
- `GetValue` gives basic functionality of getting a value of given section and key.
- Methods like `Bool`, `Int`, `Int64` return corresponding type of values.
- Methods start with `Must` return corresponding type of values and returns zero-value of given type if something goes wrong.
- `SetValue` sets value to given section and key, and inserts somewhere if it does not exist.
- `DeleteKey` deletes by given section and key.
- Finally, `SaveConfigFile` saves your configuration to local file system.
- Use method `Reload` in case someone else modified your file(s).
- Methods contains `Comment` help you manipulate comments.
- `LoadFromReader` allows loading data without an intermediate file.
- `SaveConfigData` added, which writes configuration to an arbitrary writer.
- `ReloadData` allows to reload data from memory.

Note that you cannot mix in-memory configuration with on-disk configuration.

## More Information

- All characters are CASE SENSITIVE, BE CAREFUL!

## Credits

- [goconf](http://code.google.com/p/goconf/)
- [robfig/config](https://github.com/robfig/config)
- [Delete an item from a slice](https://groups.google.com/forum/?fromgroups=#!topic/golang-nuts/lYz8ftASMQ0)

## License

This project is under Apache v2 License. See the [LICENSE](LICENSE) file for the full license text.
