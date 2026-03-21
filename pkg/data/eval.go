package data

type EvalCase struct {
	Query    string
	Expected []string
}

var EvalCases = []EvalCase{
	{
		Query:    "Looking for a senior software engineer interested in machine learning ethics and open source",
		Expected: []string{"Alice Johnson"},
	},
	{
		Query:    "Need a product manager focusing on cloud computing and sustainable tech",
		Expected: []string{"Bob Smith"},
	},
	{
		Query:    "I want a data scientist specialized in recommendation systems and NLP",
		Expected: []string{"Carol Davis"},
	},
	{
		Query:    "Looking for someone experienced in UX design with accessibility expertise",
		Expected: []string{"Eva Martinez"},
	},
	{
		Query:    "Find an AI ethicist working on bias and responsible model deployment",
		Expected: []string{"Quinn Walker"},
	},
}
