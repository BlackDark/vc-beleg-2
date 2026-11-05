// Data receipt (Datenbeleg): a receipt that exists only as a data record,
// rendered for previews and the report appendix. The record is passed as
// data (never as markup), so its content cannot inject Typst code.
#let accent = rgb("#0f766e")
#let muted = rgb("#64748b")
#let line-color = rgb("#cbd5e1")

#let money-text(cents-string) = {
  // The record keeps the amount as "12.34"; German notation is "12,34 €".
  // Non-breaking space: the amount never wraps between number and sign.
  cents-string.replace(".", ",") + "\u{00A0}€"
}
// Typst does not break a word without spaces, so a long run of characters
// would leave the card. Words longer than n characters get a zero-width
// space (a line-break opportunity) every n characters. Only the rendering
// is affected; the stored record is unchanged. n is small enough that a
// chunk of the widest glyphs (W, M) still fits its column; keys sit in a
// narrow column, hence the smaller size.
#let breakable(s, n: 12) = s.split(" ").map(w => {
  let c = w.clusters()
  if c.len() <= n { w } else { c.chunks(n).map(ch => ch.join()).join("\u{200B}") }
}).join(" ")
// Safety net: whatever still does not fit is cut at the cell border and
// never paints into the neighbouring column.
// The bottom inset keeps descenders of the last line inside the clip box.
#let cell(body) = block(width: 100%, clip: true, inset: (bottom: 3pt), body)
#let date-text(x) = {
  let p = x.split("-")
  p.at(2) + "." + p.at(1) + "." + p.at(0)
}
#let extra-text(v) = breakable(if type(v) == str { v } else { json.encode(v) }, n: 16)

#let data-receipt(rec, width: 100%) = block(width: width, stroke: 0.8pt + line-color, radius: 4pt, inset: 12pt)[
  #set text(size: 9.5pt)
  #set align(left)
  #text(size: 7.6pt, weight: "semibold", fill: accent, upper("Datenbeleg"))
  #v(-2pt)
  #text(size: 13pt, weight: "bold", breakable(rec.merchant))
  #v(2pt)
  #line(length: 100%, stroke: 0.5pt + line-color)
  #grid(
    columns: (30%, 1fr),
    column-gutter: 10pt,
    row-gutter: 6pt,
    text(fill: muted, "Datum"), date-text(rec.date),
    text(fill: muted, "Betrag"), text(weight: "bold", money-text(rec.total)),
    ..if "extra" in rec {
      rec.extra.pairs().map(((k, v)) => (cell(text(fill: muted, breakable(k, n: 8))), cell(extra-text(v)))).flatten()
    } else { () },
  )
  #v(4pt)
  #text(size: 7pt, fill: muted)[Datensatz ohne Scan oder Foto; der gespeicherte Datensatz ist der Originalbeleg.]
]
