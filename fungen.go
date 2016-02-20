package main

import (
	"flag"
	"fmt"
	"go/format"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

//go:generate $GOPATH/bin/fungen -types "Generator" -methods Filter,Each

// Generator - one generator (function and information about generate)
type Generator struct {
	name         string
	method       func(_, _, _, _ string) string
	needSync     bool
	needMapToMap bool
}

var (
	packageName = flag.String("package", "main", "(Optional) Name of the package.")
	types       = flag.String("types", "", "Comma-separated list of type names, eg. 'int,string,CustomType'. The values can themselves be colon (:) separated to specify the names of entities in the generated, eg: int:I,string:Str,CustomType:CT.")
	methods     = flag.String("methods", "", "Comma-separated list of methods to generate, eg 'Map,Filter'. By default generate all methods.")
	outputName  = flag.String("filename", "fungen_auto.go", "(Optional) Filename for generated package.")
	testrun     = flag.Bool("test", false, "whether to display the generated code instead of writing out to a file.")
	generators  = GeneratorList{
		{
			name:         "Map",
			method:       getMapFunction,
			needSync:     false,
			needMapToMap: true,
		},
		{
			name:         "PMap",
			method:       getPMapFunction,
			needSync:     true,
			needMapToMap: true,
		},
		{
			name:     "Filter",
			method:   getFilterFunction,
			needSync: false,
		},
		{
			name:     "PFilter",
			method:   getPFilterFunction,
			needSync: true,
		},
		{
			name:   "Reduce",
			method: getReduceFunction,
		},
		{
			name:   "ReduceRight",
			method: getReduceRightFunction,
		},
		{
			name:   "Take",
			method: getTakeFunction,
		},
		{
			name:   "TakeWhile",
			method: getTakeWhileFunction,
		},
		{
			name:   "Drop",
			method: getDropFunction,
		},
		{
			name:   "DropWhile",
			method: getDropWhileFunction,
		},
		{
			name:   "Each",
			method: getEachFunction,
		},
		{
			name:   "EachI",
			method: getEachIFunction,
		},
		{
			name:   "All",
			method: getAllFunction,
		},
		{
			name:   "Any",
			method: getAnyFunction,
		},
	}
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\tgen -package packageName -types Types\n")
	fmt.Fprintf(os.Stderr, "Example:\n")
	fmt.Fprintf(os.Stderr, "'fungen -package mypackage -types string,int,customType,AnotherType' will create types 'stringList []string, intList []int, customTypeList []customType, AnotherTypeList []AnotherType' with the Map, Filter, Reduce, ReduceRight, Take, TakeWhile, Drop, DropWhile, Each, EachI methods on them. Additionally, methods named MapType1Type2 will be generated on these types for the remaining types. The package of the generated file will be 'mypackage' \n\n")
	fmt.Fprintf(os.Stderr, "'fungen -types string,int:I,customType:CT,AnotherType:At' will create types 'stringList []string, IList []int, CTList []customType, AtList []AnotherType'. The 'stringList' type will have the Map, Filter, Reduce, ReduceRight, Take, TakeWhile, Drop, DropWhile, Each, EachI methods on it. Additionally, it will also have MapI, MapCt and MapAt methods. The package of the generated file will be 'main' \n\n")
	fmt.Fprintf(os.Stderr, "'fungen -methods Map,Filter -types int' will create types 'intList []int' with the Map, Filter methods on them.\n\n")

	fmt.Fprintf(os.Stderr, "Flags:\n")
	flag.PrintDefaults()
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if len(*types) == 0 {
		flag.Usage()
		os.Exit(2)
	}

	methodsMap := getMethodsMap(*methods)

	importSync := ""
	needImportSync := len(generators.Filter(func(gen Generator) bool {
		selectedMethod, _ := methodsMap[gen.name]
		return selectedMethod && gen.needSync
	})) > 0
	if needImportSync {
		importSync = `import "sync"`
	}

	src := fmt.Sprintf(`// Package %[1]s - generated by fungen; DO NOT EDIT
            package %[1]s
            
            %[2]s
			
            `, *packageName, importSync)

	typeMap := getTypeMap(*types)

	for k1, v1 := range typeMap {
		src += generate(k1, v1+"List", typeMap, methodsMap)
		src = f(src)
	}

	if *testrun {
		fmt.Println(*outputName)
		fmt.Println(src)
	} else {
		err := ioutil.WriteFile(*outputName, []byte(src), 0644)
		if err != nil {
			log.Fatalf("writing output: %s", err)
		}
	}

}

func f(s string) string {
	formatted, err := format.Source([]byte(s))
	if err != nil {
		log.Fatal(err)
	}
	return string(formatted)
}

func getFileNameForTypes(t string, m map[string]string) string {
	if len(m) == 0 {
		return t
	}
	s := t
	for k, v := range m {
		if t == k {
			continue
		}
		s += "_" + v
	}
	return s
}

func getTypeMap(targets string) map[string]string {
	m := map[string]string{}
	if targets == "" {
		return m
	}

	targetParts := strings.Split(targets, ",")
	for _, t := range targetParts {
		tParts := strings.Split(t, ":")
		if len(tParts) == 1 {
			m[tParts[0]] = tParts[0]
		} else {
			m[tParts[0]] = tParts[1]
		}
	}

	return m
}

// getMethodsMap - get selected methods from -methods option, or return all methods
func getMethodsMap(methodsStr string) map[string]bool {
	result := map[string]bool{}
	if methodsStr == "" {
		generators.Each(func(gen Generator) {
			result[gen.name] = true
		})
		return result
	}

	validMethods := map[string]bool{}
	generators.Each(func(gen Generator) {
		validMethods[gen.name] = true
	})

	for _, method := range strings.Split(methodsStr, ",") {
		if _, ok := validMethods[method]; ok {
			result[method] = true
		} else {
			log.Fatalf("Error: -method parameter '%s' is not valid", method)
		}
	}

	return result
}

func generate(typeName, listname string, m map[string]string, methodsMap map[string]bool) string {
	code := fmt.Sprintf(`
            
            // %[2]s is the type for a list that holds members of type %[1]s
            type %[2]s []%[1]s
            `, typeName, listname)

	generators.Filter(func(gen Generator) bool {
		_, ok := methodsMap[gen.name]
		return ok
	}).Each(func(gen Generator) {
		if gen.needMapToMap {
			for k, v := range m {
				targetTypeName := v
				if k == typeName {
					targetTypeName = ""
				}

				code += gen.method(listname, typeName, k, targetTypeName)
			}
		} else {
			code += gen.method(listname, typeName, "", "")
		}
	})

	return code
}

func getMapFunction(listName, typeName, targetType, targetTypeName string) string {
	targetListName := targetType + "List"
	if targetTypeName == "" {
		targetListName = listName
	}

	return fmt.Sprintf(`
        // Map%[4]s is a method on %[1]s that takes a function of type %[2]s -> %[3]s and applies it to every member of %[1]s
        func (l %[1]s) Map%[4]s(f func(%[2]s) %[3]s) %[5]s {
            l2 := make(%[5]s, len(l))
            for i, t := range l {
                l2[i] = f(t)
            }
            return l2
        }
        `, listName, typeName, targetType, strings.Title(targetTypeName), targetListName)

}

func getPMapFunction(listName, typeName, targetType, targetTypeName string) string {
	targetListName := targetType + "List"
	if targetTypeName == "" {
		targetListName = listName
	}

	return fmt.Sprintf(`
        // PMap%[4]s is similar to Map%[4]s except that it executes the function on each member in parallel.
        func (l %[1]s) PMap%[4]s(f func(%[2]s) %[3]s) %[5]s {
            wg := sync.WaitGroup{}
            l2 := make(%[5]s, len(l))
            for i, t := range l {
                wg.Add(1)
                go func(i int, t %[2]s){
                    l2[i] = f(t)
                    wg.Done()
                }(i, t)
            }
            wg.Wait()
            return l2
        }
        `, listName, typeName, targetType, strings.Title(targetTypeName), targetListName)

}

func getFilterFunction(listName, typeName, _, _ string) string {
	return fmt.Sprintf(`
        // Filter is a method on %[1]s that takes a function of type %[2]s -> bool returns a list of type %[1]s which contains all members from the original list for which the function returned true
        func (l %[1]s) Filter(f func(%[2]s) bool) %[1]s {
            l2 := []%[2]s{}
            for _, t := range l {
                if f(t) {
                    l2 = append(l2, t)
                }
            }
            return l2
        }
        `, listName, typeName)
}

func getPFilterFunction(listName, typeName, _, _ string) string {
	return fmt.Sprintf(`
        // PFilter is similar to the Filter method except that the filter is applied to all the elements in parallel. The order of resulting elements cannot be guaranteed. 
        func (l %[1]s) PFilter(f func(%[2]s) bool) %[1]s {
            wg := sync.WaitGroup{}
            mutex := sync.Mutex{}
            l2 := []%[2]s{}
            for _, t := range l {
                wg.Add(1)
                go func(t %[2]s){
                    if f(t) {
                        mutex.Lock()
                        l2 = append(l2, t)
                        mutex.Unlock()
                    }            
                    wg.Done()
                }(t)
            }
            wg.Wait()
            return l2
        }
        `, listName, typeName)
}

func getEachFunction(listName, typeName, _, _ string) string {
	return fmt.Sprintf(`
        // Each is a method on %[1]s that takes a function of type %[2]s -> void and applies the function to each member of the list and then returns the original list.
        func (l %[1]s) Each(f func(%[2]s)) %[1]s {
            for _, t := range l {
                f(t) 
            }
            return l
        }
        `, listName, typeName)
}

func getEachIFunction(listName, typeName, _, _ string) string {
	return fmt.Sprintf(`
        // EachI is a method on %[1]s that takes a function of type (int, %[2]s) -> void and applies the function to each member of the list and then returns the original list. The int parameter to the function is the index of the element.
        func (l %[1]s) EachI(f func(int, %[2]s)) %[1]s {
            for i, t := range l {
                f(i, t) 
            }
            return l
        }
        `, listName, typeName)
}

func getDropWhileFunction(listName, typeName, _, _ string) string {
	return fmt.Sprintf(`
        // DropWhile is a method on %[1]s that takes a function of type %[2]s -> bool and returns a list of type %[1]s which excludes the first members from the original list for which the function returned true
        func (l %[1]s) DropWhile(f func(%[2]s) bool) %[1]s {
            for i, t := range l {
                if !f(t) {
                    return l[i:]
                }
            }
            var l2 %[1]s
            return l2
        }
        `, listName, typeName)
}

func getTakeWhileFunction(listName, typeName, _, _ string) string {
	return fmt.Sprintf(`
        // TakeWhile is a method on %[1]s that takes a function of type %[2]s -> bool and returns a list of type %[1]s which includes only the first members from the original list for which the function returned true
        func (l %[1]s) TakeWhile(f func(%[2]s) bool) %[1]s {
            for i, t := range l {
                if !f(t) {
                    return l[:i]
                }
            }
            return l
        }
        `, listName, typeName)
}

func getTakeFunction(listName, typeName, _, _ string) string {
	return fmt.Sprintf(`
        // Take is a method on %[1]s that takes an integer n and returns the first n elements of the original list. If the list contains fewer than n elements then the entire list is returned.
        func (l %[1]s) Take(n int) %[1]s {
            if len(l) >= n {
                return l[:n]
            }
            return l
        }
        `, listName, typeName)
}

func getDropFunction(listName, typeName, _, _ string) string {
	return fmt.Sprintf(`
        // Drop is a method on %[1]s that takes an integer n and returns all but the first n elements of the original list. If the list contains fewer than n elements then an empty list is returned.
        func (l %[1]s) Drop(n int) %[1]s {
            if len(l) >= n {
                return l[n:]
            }
            var l2 %[1]s
            return l2
        }
        `, listName, typeName)
}

func getReduceFunction(listName, typename, _, _ string) string {
	return fmt.Sprintf(`
        // Reduce is a method on %[1]s that takes a function of type (%[2]s, %[2]s) -> %[2]s and returns a %[2]s which is the result of applying the function to all members of the original list starting from the first member
        func (l %[1]s) Reduce(t1 %[2]s, f func(%[2]s, %[2]s) %[2]s) %[2]s {
            for _, t := range l {
                t1 = f(t1, t)
            }
            return t1
        }
        `, listName, typename)
}

func getReduceRightFunction(listName, typename, _, _ string) string {
	return fmt.Sprintf(`
        // ReduceRight is a method on %[1]s that takes a function of type (%[2]s, %[2]s) -> %[2]s and returns a %[2]s which is the result of applying the function to all members of the original list starting from the last member
        func (l %[1]s) ReduceRight(t1 %[2]s, f func(%[2]s, %[2]s) %[2]s) %[2]s {
            for i := len(l) - 1; i >= 0; i-- {
                t := l[i]
                t1 = f(t, t1)
            }
            return t1
        }
        `, listName, typename)
}

func getAllFunction(listName, typename, _, _ string) string {
	return fmt.Sprintf(`
        // All is a method on %[1]s that returns true if all the members of the list satisfy a function or if the list is empty. 
        func (l %[1]s) All(f func(%[2]s) bool) bool {
            for _, t := range l {
                if !f(t) {
                    return false
                }
            }
            return true
        }
        `, listName, typename)
}

func getAnyFunction(listName, typename, _, _ string) string {
	return fmt.Sprintf(`
        // Any is a method on %[1]s that returns true if at least one member of the list satisfies a function. It returns false if the list is empty. 
        func (l %[1]s) Any(f func(%[2]s) bool) bool {
            for _, t := range l {
                if f(t) {
                    return true
                }
            }
            return false
        }
        `, listName, typename)
}
