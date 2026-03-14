package main

import "fmt"

func doSomething() {
    fmt.Println("Did something")
}

func TestFunction() {
    x := 10
    if x > 5 {
        fmt.Println("Greater")
    }

    doSomething()

    var y int = 10
    _ = y

    for i := 0; i < 5; i++ {
        fmt.Println("Loop", i)
    }

    if true {
        fmt.Println("Always")
    } else {
        fmt.Println("Else")
    }
}
