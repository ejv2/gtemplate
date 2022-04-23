// Copyright 2022 Ethan Marshall.
// Licensed under the ISC licence - see COPYING.

/*
Package gtemplate provides an http.Handler compatable handler for
html/template formatted documents. Template documents will be loaded
once and cached for reuse with multiple sets of data. See documentation
on DataBroker.

A gtemplate TemplateServer can be used as a handler for http.Server like
so:

	hndl, err := gtemplate.NewServer("public/", broker)
	if err != nil {
		panic(err)
	}
	http.ListenAndServe("localhost:8080", hndl)

It can also be used in http.Handle for a path:

	hndl, err := gtemplate.NewServer("public/content/", broker)
	if err != nil {
		panic(err)
	}
	http.Handle("/content/", hndl)

In these examples, "broker" is used as a substitute for a data broker,
which is simply a type capable of supplying an arbitrary map of string
keys to any type for usage in the template. In the test suite, an example
of this handling is used to display a string, date and do a conditional
template.
*/
package gtemplate
