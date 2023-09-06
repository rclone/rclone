/*
Package hdfs provides a native, idiomatic interface to HDFS. Where possible,
it mimics the functionality and signatures of the standard `os` package.

Example:

	client, _ := hdfs.New("namenode:8020")

	file, _ := client.Open("/mobydick.txt")

	buf := make([]byte, 59)
	file.ReadAt(buf, 48847)

	fmt.Println(string(buf))
	// => Abominable are the tumblers into which he pours his poison.
*/
package hdfs
