package main

func findProduct(id int) (int, bool) {
	for i, val := range products {
		if id == val.Id {
			return i, true
		}
	}
	return -1, false
}
