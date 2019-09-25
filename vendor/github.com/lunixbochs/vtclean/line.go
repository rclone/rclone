package vtclean

type char struct {
	char  byte
	vt100 []byte
}

func chars(p []byte) []char {
	tmp := make([]char, len(p))
	for i, v := range p {
		tmp[i].char = v
	}
	return tmp
}

type lineEdit struct {
	buf       []char
	pos, size int
	vt100     []byte
}

func newLineEdit(length int) *lineEdit {
	return &lineEdit{buf: make([]char, length)}
}

func (l *lineEdit) Vt100(p []byte) {
	l.vt100 = p
}

func (l *lineEdit) Move(x int) {
	if x < 0 && l.pos <= -x {
		l.pos = 0
	} else if x > 0 && l.pos+x > l.size {
		l.pos = l.size
	} else {
		l.pos += x
	}
}

func (l *lineEdit) MoveAbs(x int) {
	if x < l.size {
		l.pos = x
	}
}

func (l *lineEdit) Write(p []byte) {
	c := chars(p)
	if len(c) > 0 {
		c[0].vt100 = l.vt100
		l.vt100 = nil
	}
	if len(l.buf)-l.pos < len(c) {
		l.buf = append(l.buf[:l.pos], c...)
	} else {
		copy(l.buf[l.pos:], c)
	}
	l.pos += len(c)
	if l.pos > l.size {
		l.size = l.pos
	}
}

func (l *lineEdit) Insert(p []byte) {
	c := chars(p)
	if len(c) > 0 {
		c[0].vt100 = l.vt100
		l.vt100 = nil
	}
	l.size += len(c)
	c = append(c, l.buf[l.pos:]...)
	l.buf = append(l.buf[:l.pos], c...)
}

func (l *lineEdit) Delete(n int) {
	most := l.size - l.pos
	if n > most {
		n = most
	}
	copy(l.buf[l.pos:], l.buf[l.pos+n:])
	l.size -= n
}

func (l *lineEdit) Clear() {
	for i := 0; i < len(l.buf); i++ {
		l.buf[i].char = ' '
	}
}
func (l *lineEdit) ClearLeft() {
	for i := 0; i < l.pos+1; i++ {
		l.buf[i].char = ' '
	}
}
func (l *lineEdit) ClearRight() {
	l.size = l.pos
}

func (l *lineEdit) Bytes() []byte {
	length := 0
	buf := l.buf[:l.size]
	for _, v := range buf {
		length += 1 + len(v.vt100)
	}
	tmp := make([]byte, 0, length)
	for _, v := range buf {
		tmp = append(tmp, v.vt100...)
		tmp = append(tmp, v.char)
	}
	return tmp
}

func (l *lineEdit) String() string {
	return string(l.Bytes())
}
