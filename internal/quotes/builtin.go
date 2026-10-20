// Package quotes contains short, source-attributed quotations suitable for the IDIOS quote library.
package quotes

import "idios/internal/domain"

// Builtin returns a fresh copy of the starter collection. Text is kept in its original English;
// every item records its work and a public reference so the attribution can be checked.
func Builtin() []domain.Quote {
	return []domain.Quote{
		{
			Text: "Premature optimization is the root of all evil.", Author: "Donald E. Knuth",
			Source:    "Structured Programming with go to Statements, ACM Computing Surveys 6(4), 1974, p. 268",
			SourceURL: "https://doi.org/10.1145/356635.356640", Category: "Software engineering",
		},
		{
			Text: "Adding manpower to a late software project makes it later.", Author: "Frederick P. Brooks Jr.",
			Source:    "The Mythical Man-Month: Essays on Software Engineering, Chapter 2 (1975)",
			SourceURL: "https://www.cs.cmu.edu/afs/cs.cmu.edu/academic/class/15712-f17/www/papers/mythicalmanmonth00fred.pdf", Category: "Software engineering",
		},
		{
			Text: "Program testing can at best show the presence of errors but never their absence.", Author: "Edsger W. Dijkstra",
			Source:    "The Humble Programmer (EWD 340), 1972",
			SourceURL: "https://www.cs.utexas.edu/~EWD/transcriptions/EWD03xx/EWD340.html", Category: "Software engineering",
		},
		{
			Text: "The language itself becomes part of the problem rather than part of its solution.", Author: "C. A. R. (Tony) Hoare",
			Source:    "The Emperor's Old Clothes, Communications of the ACM 24(2), 1981",
			SourceURL: "https://www.cs.princeton.edu/courses/archive/spring10/cos333/hoare.turing.pdf", Category: "Software design",
		},
		{
			Text: "It is easier to write an incorrect program than understand a correct one.", Author: "Alan J. Perlis",
			Source:    "Epigrams in Programming (1982)",
			SourceURL: "https://cs.yale.edu/homes/perlis-alan/quotes.html", Category: "Programming",
		},
		{
			Text: "Mathematics, rightly viewed, possesses not only truth, but supreme beauty.", Author: "Bertrand Russell",
			Source:    "The Study of Mathematics, in Mysticism and Logic and Other Essays (1917)",
			SourceURL: "https://www.gutenberg.org/cache/epub/25447/pg25447-images.html", Category: "Mathematics",
		},
		{
			Text: "We must know, we will know.", Author: "David Hilbert",
			Source:    "Address to the Society of German Scientists and Physicians, Königsberg, 1930",
			SourceURL: "https://mathweb.ucsd.edu/~williams/motiv/hilbert.html", Category: "Mathematics",
		},
		{
			Text: "A mathematician, like a painter or a poet, is a maker of patterns.", Author: "G. H. Hardy",
			Source:    "A Mathematician's Apology (1940), Chapter 10",
			SourceURL: "https://doi.org/10.1017/CBO9781139644112.012", Category: "Mathematics",
		},
		{
			Text: "The purpose of computing is insight, not numbers.", Author: "Richard W. Hamming",
			Source:    "Numerical Methods for Scientists and Engineers (1962); also collected in Hamming quotations",
			SourceURL: "https://mathshistory.st-andrews.ac.uk/Biographies/Hamming/quotations/", Category: "Computing",
		},
		{
			Text: "We can only see a short distance ahead, but we can see plenty there that needs to be done.", Author: "Alan M. Turing",
			Source:    "Computing Machinery and Intelligence, Mind 59(236), 1950",
			SourceURL: "https://www.cs.sfu.ca/~vaughan/teaching/889/papers/turing1950.html", Category: "Computing",
		},
		{
			Text: "An approximate answer to the right problem is worth a good deal more than an exact answer to an approximate problem.", Author: "John W. Tukey",
			Source:    "The Future of Data Analysis, Annals of Mathematical Statistics 33(1), 1962",
			SourceURL: "https://doi.org/10.1214/aoms/1177704711", Category: "Mathematics and statistics",
		},
	}
}
