package goquery

import (
	"bytes"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

var rxClassTrim = regexp.MustCompile("[\t\r\n]")

// Attr gets the specified attribute's value for the first element in the
// Selection. To get the value for each element individually, use a looping
// construct such as Each or Map method.
func (s *Selection) Attr(attrName string) (val string, exists bool) {
	if len(s.Nodes) == 0 {
		return
	}
	return getAttributeValue(attrName, s.Nodes[0])
}

// AttrOr works like Attr but returns default value if attribute is not present.
func (s *Selection) AttrOr(attrName, defaultValue string) string {
	if len(s.Nodes) == 0 {
		return defaultValue
	}

	val, exists := getAttributeValue(attrName, s.Nodes[0])
	if !exists {
		return defaultValue
	}

	return val
}

// RemoveAttr removes the named attribute from each element in the set of matched elements.
func (s *Selection) RemoveAttr(attrName string) *Selection {
	for _, n := range s.Nodes {
		removeAttr(n, attrName)
	}

	return s
}

// SetAttr sets the given attribute on each element in the set of matched elements.
func (s *Selection) SetAttr(attrName, val string) *Selection {
	for _, n := range s.Nodes {
		attr := getAttributePtr(attrName, n)
		if attr == nil {
			n.Attr = append(n.Attr, html.Attribute{Key: attrName, Val: val})
		} else {
			attr.Val = val
		}
	}

	return s
}

// Text gets the combined text contents of each element in the set of matched
// elements, including their descendants.
func (s *Selection) Text() string {
	var buf bytes.Buffer

	// Slightly optimized vs calling Each: no single selection object created
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.TextNode {
			// Keep newlines and spaces, like jQuery
			buf.WriteString(n.Data)
		}
		if n.FirstChild != nil {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				f(c)
			}
		}
	}
	for _, n := range s.Nodes {
		f(n)
	}

	return buf.String()
}

// Size is an alias for Length.
func (s *Selection) Size() int {
	return s.Length()
}

// Length returns the number of elements in the Selection object.
func (s *Selection) Length() int {
	return len(s.Nodes)
}

// Html gets the HTML contents of the first element in the set of matched
// elements. It includes text and comment nodes.
func (s *Selection) Html() (ret string, e error) {
	// Since there is no .innerHtml, the HTML content must be re-created from
	// the nodes using html.Render.
	var buf bytes.Buffer

	if len(s.Nodes) > 0 {
		for c := s.Nodes[0].FirstChild; c != nil; c = c.NextSibling {
			e = html.Render(&buf, c)
			if e != nil {
				return
			}
		}
		ret = buf.String()
	}

	return
}

// AddClass adds the given class(es) to each element in the set of matched elements.
// Multiple class names can be specified, separated by a space or via multiple arguments.
func (s *Selection) AddClass(class ...string) *Selection {
	classStr := strings.TrimSpace(strings.Join(class, " "))

	if classStr == "" {
		return s
	}

	tcls := getClassesSlice(classStr)
	for _, n := range s.Nodes {
		curClasses, attr := getClassesAndAttr(n, true)
		for _, newClass := range tcls {
			if !strings.Contains(curClasses, " "+newClass+" ") {
				curClasses += newClass + " "
			}
		}

		setClasses(n, attr, curClasses)
	}

	return s
}

// HasClass determines whether any of the matched elements are assigned the
// given class.
func (s *Selection) HasClass(class string) bool {
	class = " " + class + " "
	for _, n := range s.Nodes {
		classes, _ := getClassesAndAttr(n, false)
		if strings.Contains(classes, class) {
			return true
		}
	}
	return false
}

// RemoveClass removes the given class(es) from each element in the set of matched elements.
// Multiple class names can be specified, separated by a space or via multiple arguments.
// If no class name is provided, all classes are removed.
func (s *Selection) RemoveClass(class ...string) *Selection {
	var rclasses []string

	classStr := strings.TrimSpace(strings.Join(class, " "))
	remove := classStr == ""

	if !remove {
		rclasses = getClassesSlice(classStr)
	}

	for _, n := range s.Nodes {
		if remove {
			removeAttr(n, "class")
		} else {
			classes, attr := getClassesAndAttr(n, true)
			for _, rcl := range rclasses {
				classes = strings.Replace(classes, " "+rcl+" ", " ", -1)
			}

			setClasses(n, attr, classes)
		}
	}

	return s
}

// ToggleClass adds or removes the given class(es) for each element in the set of matched elements.
// Multiple class names can be specified, separated by a space or via multiple arguments.
func (s *Selection) ToggleClass(class ...string) *Selection {
	classStr := strings.TrimSpace(strings.Join(class, " "))

	if classStr == "" {
		return s
	}

	tcls := getClassesSlice(classStr)

	for _, n := range s.Nodes {
		classes, attr := getClassesAndAttr(n, true)
		for _, tcl := range tcls {
			if strings.Contains(classes, " "+tcl+" ") {
				classes = strings.Replace(classes, " "+tcl+" ", " ", -1)
			} else {
				classes += tcl + " "
			}
		}

		setClasses(n, attr, classes)
	}

	return s
}

func getAttributePtr(attrName string, n *html.Node) *html.Attribute {
	if n == nil {
		return nil
	}

	for i, a := range n.Attr {
		if a.Key == attrName {
			return &n.Attr[i]
		}
	}
	return nil
}

// Private function to get the specified attribute's value from a node.
func getAttributeValue(attrName string, n *html.Node) (val string, exists bool) {
	if a := getAttributePtr(attrName, n); a != nil {
		val = a.Val
		exists = true
	}
	return
}

// Get and normalize the "class" attribute from the node.
func getClassesAndAttr(n *html.Node, create bool) (classes string, attr *html.Attribute) {
	// Applies only to element nodes
	if n.Type == html.ElementNode {
		attr = getAttributePtr("class", n)
		if attr == nil && create {
			n.Attr = append(n.Attr, html.Attribute{
				Key: "class",
				Val: "",
			})
			attr = &n.Attr[len(n.Attr)-1]
		}
	}

	if attr == nil {
		classes = " "
	} else {
		classes = rxClassTrim.ReplaceAllString(" "+attr.Val+" ", " ")
	}

	return
}

func getClassesSlice(classes string) []string {
	return strings.Split(rxClassTrim.ReplaceAllString(" "+classes+" ", " "), " ")
}

func removeAttr(n *html.Node, attrName string) {
	for i, a := range n.Attr {
		if a.Key == attrName {
			n.Attr[i], n.Attr[len(n.Attr)-1], n.Attr =
				n.Attr[len(n.Attr)-1], html.Attribute{}, n.Attr[:len(n.Attr)-1]
			return
		}
	}
}

func setClasses(n *html.Node, attr *html.Attribute, classes string) {
	classes = strings.TrimSpace(classes)
	if classes == "" {
		removeAttr(n, "class")
		return
	}

	attr.Val = classes
}
