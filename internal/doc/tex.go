package doc

import (
	"regexp"
	"strings"
)

// texToUnicode approximates TeX as plain text, for when equations can't be
// rendered as images.

var texSymbols = map[string]string{
	"alpha": "α", "beta": "β", "gamma": "γ", "delta": "δ", "epsilon": "ε", "varepsilon": "ε", "zeta": "ζ",
	"eta": "η", "theta": "θ", "iota": "ι", "kappa": "κ", "lambda": "λ", "mu": "μ", "nu": "ν", "xi": "ξ",
	"pi": "π", "rho": "ρ", "sigma": "σ", "tau": "τ", "upsilon": "υ", "phi": "φ", "varphi": "φ", "chi": "χ",
	"psi": "ψ", "omega": "ω", "Gamma": "Γ", "Delta": "Δ", "Theta": "Θ", "Lambda": "Λ", "Xi": "Ξ", "Pi": "Π",
	"Sigma": "Σ", "Phi": "Φ", "Psi": "Ψ", "Omega": "Ω",
	"sum": "∑", "prod": "∏", "int": "∫", "oint": "∮", "infty": "∞", "partial": "∂", "nabla": "∇",
	"cdot": "·", "times": "×", "div": "÷", "pm": "±", "mp": "∓", "leq": "≤", "le": "≤", "geq": "≥", "ge": "≥",
	"neq": "≠", "ne": "≠", "approx": "≈", "equiv": "≡", "sim": "∼", "propto": "∝",
	"to": "→", "rightarrow": "→", "leftarrow": "←", "Rightarrow": "⇒", "Leftarrow": "⇐", "iff": "⇔",
	"leftrightarrow": "↔", "mapsto": "↦", "in": "∈", "notin": "∉", "subset": "⊂", "subseteq": "⊆",
	"supset": "⊃", "cup": "∪", "cap": "∩", "emptyset": "∅", "forall": "∀", "exists": "∃", "neg": "¬",
	"land": "∧", "lor": "∨", "sqrt": "√", "ldots": "…", "cdots": "⋯", "dots": "…", "circ": "∘",
	"mathbb": "", "mathrm": "", "mathbf": "", "text": "", "left": "", "right": "", "quad": " ", "qquad": "  ",
	"R": "ℝ", "N": "ℕ", "Z": "ℤ", "Q": "ℚ", "C": "ℂ",
}

var (
	texFrac = regexp.MustCompile(`\\frac\{([^{}]*)\}\{([^{}]*)\}`)
	texCmd  = regexp.MustCompile(`\\([a-zA-Z]+)`)
	texSup  = regexp.MustCompile(`\^\{([0-9n+\-=()i]+)\}|\^([0-9n+\-=()i])`)
	texSub  = regexp.MustCompile(`_\{([0-9+\-=()]+)\}|_([0-9])`)
	sup     = strings.NewReplacer("0", "⁰", "1", "¹", "2", "²", "3", "³", "4", "⁴", "5", "⁵", "6", "⁶", "7", "⁷", "8", "⁸", "9", "⁹", "+", "⁺", "-", "⁻", "=", "⁼", "(", "⁽", ")", "⁾", "n", "ⁿ", "i", "ⁱ")
	sub     = strings.NewReplacer("0", "₀", "1", "₁", "2", "₂", "3", "₃", "4", "₄", "5", "₅", "6", "₆", "7", "₇", "8", "₈", "9", "₉", "+", "₊", "-", "₋", "=", "₌", "(", "₍", ")", "₎")
)

func texToUnicode(s string) string {
	s = texFrac.ReplaceAllString(s, "($1)/($2)")
	s = texSup.ReplaceAllStringFunc(s, func(m string) string {
		return sup.Replace(strings.Trim(m, "^{}"))
	})
	s = texSub.ReplaceAllStringFunc(s, func(m string) string {
		return sub.Replace(strings.Trim(m, "_{}"))
	})
	s = texCmd.ReplaceAllStringFunc(s, func(m string) string {
		if r, ok := texSymbols[m[1:]]; ok {
			return r
		}
		return m
	})
	return strings.NewReplacer("{", "", "}", "", `\,`, " ", `\;`, " ", `\!`, "", `\\`, " ").Replace(s)
}
