package kingpin

import (
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"
)

func (c *cmdMixin) fromStruct(clause *CmdClause, v interface{}) error { // nolint: gocyclo
	urv := reflect.ValueOf(v)
	rv := reflect.Indirect(reflect.ValueOf(v))
	if rv.Kind() != reflect.Struct || !rv.CanSet() {
		return fmt.Errorf("expected a pointer to a struct but got a " + urv.Type().String())
	}
	for i := 0; i < rv.NumField(); i++ {
		// Parse out tags
		field := rv.Field(i)
		ft := rv.Type().Field(i)
		if strings.ToLower(ft.Name[0:1]) == ft.Name[0:1] {
			continue
		}
		tag := ft.Tag
		help := tag.Get("help")
		if help == "" {
			help = tag.Get("description")
		}
		placeholder := tag.Get("placeholder")
		if placeholder == "" {
			placeholder = tag.Get("value-name")
		}
		dflt := tag.Get("default")
		short := tag.Get("short")
		required := tag.Get("required")
		hidden := tag.Get("hidden")
		env := tag.Get("env")
		enum := tag.Get("enum")
		name := strings.ToLower(strings.Join(camelCase(ft.Name), "-"))
		if tag.Get("long") != "" {
			name = tag.Get("long")
		}
		arg := tag.Get("arg")
		format := tag.Get("format") // For time.Time only
		if format == "" {
			format = time.RFC3339
		}

		var action Action
		onMethodName := "On" + strings.ToUpper(ft.Name[0:1]) + ft.Name[1:]
		if actionMethod := urv.MethodByName(onMethodName); actionMethod.IsValid() {
			action, _ = actionMethod.Interface().(func(element *ParseElement, context *ParseContext) error)
		}

		if field.Kind() == reflect.Struct && ft.Type != reflect.TypeOf(time.Time{}) {
			if ft.Anonymous {
				if err := c.fromStruct(clause, field.Addr().Interface()); err != nil {
					return err
				}
			} else {
				cmd := c.addCommand(name, help)
				cmd.parent = clause
				if hidden != "" {
					cmd = cmd.Hidden()
				}
				if err := cmd.Struct(field.Addr().Interface()); err != nil {
					return err
				}
			}
			continue
		}

		// Define flag using extracted tags
		var clause *Clause
		if arg != "" {
			clause = c.Arg(name, help)
		} else {
			clause = c.Flag(name, help)
		}
		if action != nil {
			clause.Action(action)
		}
		if dflt != "" {
			clause = clause.Default(dflt)
		}
		if short != "" {
			r, _ := utf8.DecodeRuneInString(short)
			if r == utf8.RuneError {
				return fmt.Errorf("invalid short flag %s", short)
			}
			clause = clause.Short(r)
		}
		if required != "" {
			clause = clause.Required()
		}
		if hidden != "" {
			clause = clause.Hidden()
		}
		if placeholder != "" {
			clause = clause.PlaceHolder(placeholder)
		}
		if env != "" {
			clause = clause.Envar(env)
		}
		ptr := field.Addr().Interface()
		if ft.Type == reflect.TypeOf(&url.URL{}) {
			clause.URLVar(ptr.(**url.URL))
		} else if ft.Type == reflect.TypeOf(time.Duration(0)) {
			clause.DurationVar(ptr.(*time.Duration))
		} else if ft.Type == reflect.TypeOf(time.Time{}) {
			clause.TimeVar(format, ptr.(*time.Time))
		} else {
			switch ft.Type.Kind() {
			case reflect.String:
				if enum != "" {
					clause.EnumVar(ptr.(*string), strings.Split(enum, ",")...)
				} else {
					clause.StringVar(ptr.(*string))
				}

			case reflect.Bool:
				clause.BoolVar(ptr.(*bool))

			case reflect.Float32:
				clause.Float32Var(ptr.(*float32))
			case reflect.Float64:
				clause.Float64Var(ptr.(*float64))

			case reflect.Int:
				clause.IntVar(ptr.(*int))
			case reflect.Int8:
				clause.Int8Var(ptr.(*int8))
			case reflect.Int16:
				clause.Int16Var(ptr.(*int16))
			case reflect.Int32:
				clause.Int32Var(ptr.(*int32))
			case reflect.Int64:
				clause.Int64Var(ptr.(*int64))

			case reflect.Uint:
				clause.UintVar(ptr.(*uint))
			case reflect.Uint8:
				clause.Uint8Var(ptr.(*uint8))
			case reflect.Uint16:
				clause.Uint16Var(ptr.(*uint16))
			case reflect.Uint32:
				clause.Uint32Var(ptr.(*uint32))
			case reflect.Uint64:
				clause.Uint64Var(ptr.(*uint64))

			case reflect.Slice:
				if ft.Type == reflect.TypeOf([]*url.URL{}) {
					clause.URLListVar(ptr.(*[]*url.URL))
				} else if ft.Type == reflect.TypeOf(time.Duration(0)) {
					clause.DurationListVar(ptr.(*[]time.Duration))
				} else if ft.Type == reflect.TypeOf(time.Time{}) {
					clause.TimeListVar(format, ptr.(*[]time.Time))
				} else {
					switch ft.Type.Elem().Kind() {
					case reflect.String:
						if enum != "" {
							clause.EnumsVar(field.Addr().Interface().(*[]string), strings.Split(enum, ",")...)
						} else {
							clause.StringsVar(field.Addr().Interface().(*[]string))
						}

					case reflect.Bool:
						clause.BoolListVar(field.Addr().Interface().(*[]bool))

					case reflect.Float32:
						clause.Float32ListVar(ptr.(*[]float32))
					case reflect.Float64:
						clause.Float64ListVar(ptr.(*[]float64))

					case reflect.Int:
						clause.IntsVar(field.Addr().Interface().(*[]int))
					case reflect.Int8:
						clause.Int8ListVar(ptr.(*[]int8))
					case reflect.Int16:
						clause.Int16ListVar(ptr.(*[]int16))
					case reflect.Int32:
						clause.Int32ListVar(ptr.(*[]int32))
					case reflect.Int64:
						clause.Int64ListVar(ptr.(*[]int64))

					case reflect.Uint:
						clause.UintsVar(ptr.(*[]uint))
					case reflect.Uint8:
						clause.HexBytesVar(ptr.(*[]byte))
					case reflect.Uint16:
						clause.Uint16ListVar(ptr.(*[]uint16))
					case reflect.Uint32:
						clause.Uint32ListVar(ptr.(*[]uint32))
					case reflect.Uint64:
						clause.Uint64ListVar(ptr.(*[]uint64))

					default:
						return fmt.Errorf("unsupported field type %s for field %s", ft.Type.String(), ft.Name)
					}
				}

			default:
				return fmt.Errorf("unsupported field type %s for field %s", ft.Type.String(), ft.Name)
			}
		}
	}
	return nil
}
