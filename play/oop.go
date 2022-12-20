package play

import (
	"fmt"
)

type Person interface {
	FullName() string
}

type Employee struct {
	FirstName   string
	LastName    string
	TotalLeaves int
	LeavesTaken int
}

func (e *Employee) FullName() string {
	return e.FirstName + " " + e.LastName
}

type Manager struct {
	Employee
	Level int
}

func (e *Manager) FullName() string {
	return e.FirstName + " " + e.LastName + "()"
}

func main() {
	a := Manager{Employee{"JUN", "ZHANG", 0, 1}, 2}
	fmt.Println(a.FullName())
}
