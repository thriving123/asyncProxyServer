package util

import goerrors "errors"

func OkOrPanic(err error, wrapErrors ...error) {
	if err == nil {
		return
	}
	if len(wrapErrors) == 0 {
		panic(err)
	}
	var finalError error
	for _, e := range wrapErrors {
		finalError = goerrors.Join(err, e)
	}
	panic(finalError)
}
