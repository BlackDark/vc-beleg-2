// Monthly meal allowance report. Data comes from data.json (the frozen
// snapshot); user content is only ever passed as data.
#let s = json("data.json")
#let r = s.result
#let t = r.totals
#let accent = rgb("#0f766e")
#let muted = rgb("#64748b")
#let line-color = rgb("#e2e8f0")
#let soft = rgb("#f1f5f9")

#let month-names = ("Januar", "Februar", "März", "April", "Mai", "Juni", "Juli", "August", "September", "Oktober", "November", "Dezember")
#let month-title = month-names.at(int(s.month.slice(5, 7)) - 1) + " " + s.month.slice(0, 4)
#let weekdays = ("Mo", "Di", "Mi", "Do", "Fr", "Sa", "So")

#set document(title: "Essenszuschuss " + month-title, author: s.employeeName)
#set text(font: "Inter", size: 9pt, lang: "de", fill: rgb("#0f172a"))
#set par(leading: 0.55em)
#set page(
  paper: "a4",
  margin: (top: 22mm, bottom: 20mm, x: 17mm),
  header: context {
    if counter(page).get().first() > 1 {
      set text(size: 7.5pt, fill: muted)
      grid(columns: (1fr, auto), s.employeeName + " · " + s.company.name, "Essenszuschuss " + month-title + " · Version " + str(s.version))
      v(-4pt)
      line(length: 100%, stroke: 0.5pt + line-color)
    }
  },
  footer: context {
    set text(size: 7.5pt, fill: muted)
    grid(columns: (1fr, auto),
      "Erstellt am " + s.generatedAt,
      "Seite " + str(counter(page).get().first()) + " / " + str(counter(page).final().first()))
  },
)

#let money(c) = {
  let neg = c < 0
  let v = calc.abs(c)
  let euros = str(calc.quo(v, 100))
  let cents = calc.rem(v, 100)
  let parts = ()
  while euros.len() > 3 {
    parts.insert(0, euros.slice(euros.len() - 3))
    euros = euros.slice(0, euros.len() - 3)
  }
  parts.insert(0, euros)
  (if neg { "−" } else { "" }) + parts.join(".") + "," + (if cents < 10 { "0" } else { "" }) + str(cents) + " €"
}
#let date(x) = {
  if x == none or x == "" { return "" }
  let p = x.slice(0, 10).split("-")
  p.at(2) + "." + p.at(1) + "." + p.at(0)
}
#let weekday(x) = {
  let p = x.split("-").map(int)
  weekdays.at(datetime(year: p.at(0), month: p.at(1), day: p.at(2)).weekday() - 1)
}
#let pct(bp) = {
  let whole = str(calc.quo(bp, 100))
  let frac = calc.rem(bp, 100)
  if frac == 0 { whole + " %" } else { whole + "," + (if frac < 10 { "0" } else { "" }) + str(frac).trim("0", at: end) + " %" }
}
#let receipt(id) = s.receipts.find(x => x.id == id)

#let section(title) = {
  v(10pt)
  block(below: 6pt, text(size: 12pt, weight: "semibold", fill: accent, title))
}

#let data-table(columns: (), align-right: (), header: (), rows: (), footer: none) = {
  set text(size: 8.3pt)
  table(
    columns: columns,
    stroke: (x, y) => (bottom: if y == 0 { 0.8pt + accent } else { 0.4pt + line-color }),
    fill: (x, y) => if y > 0 and calc.even(y) { soft } else { none },
    inset: (x: 5pt, y: 4.5pt),
    align: (x, y) => if align-right.contains(x) { right + horizon } else { left + horizon },
    table.header(..header.map(h => text(weight: "semibold", fill: muted, size: 7.6pt, upper(h)))),
    ..rows.flatten(),
    ..if footer != none { footer.map(c => text(weight: "semibold", c)) } else { () },
  )
}

#let model-label = (
  A: "A · Pauschalversteuerung durch den Arbeitgeber",
  B: "B · individuelle Versteuerung über die Lohnabrechnung",
  C: "C · steuerfreier Zuschuss bis zum Aufstockungsbetrag",
)
#let warning-label = (
  weekend: "Wochenende",
  holiday: "Feiertag",
  several_receipts: "mehrere Belege",
  over_limit: "nicht angesetzt (über Tagesgrenze)",
)

// ── Title ──────────────────────────────────────────────────────────────
#grid(
  columns: (1fr, auto),
  align: (left + bottom, right + bottom),
  [
    #text(size: 8pt, fill: muted, upper("Essenszuschuss · Monatsbericht"))
    #v(-4pt)
    #text(size: 20pt, weight: "bold", month-title)
  ],
  box(fill: soft, inset: (x: 10pt, y: 7pt), radius: 4pt)[
    #set text(size: 8pt)
    #grid(columns: (auto, auto), column-gutter: 8pt, row-gutter: 4pt, align: (left, right),
      text(fill: muted, "Version"), text(weight: "semibold", str(s.version)),
      text(fill: muted, "Modell"), r.model,
      text(fill: muted, "Tagesgrenze"), str(r.dayLimit) + " Tage",
    )
  ],
)
#v(8pt)
#line(length: 100%, stroke: 1.2pt + accent)
#v(6pt)

#grid(
  columns: (1fr, 1fr),
  column-gutter: 18pt,
  [
    #text(size: 7.6pt, weight: "semibold", fill: muted, upper("Arbeitgeber"))
    #v(-2pt)
    #text(size: 10.5pt, weight: "semibold", s.company.name) \
    #s.company.address.replace("\n", ", ")
  ],
  [
    #text(size: 7.6pt, weight: "semibold", fill: muted, upper("Mitarbeiter"))
    #v(-2pt)
    #text(size: 10.5pt, weight: "semibold", s.employeeName)
  ],
)
#if s.company.legalBasis != "" {
  v(6pt)
  text(fill: muted, "Rechtsgrundlage: ") + s.company.legalBasis
}
#if s.reopenReason != "" {
  v(4pt)
  block(fill: rgb("#fef9c3"), inset: 7pt, radius: 3pt, width: 100%, text(size: 8pt)[*Korrektur gegenüber Version #(s.version - 1):* #s.reopenReason])
}

// ── Summary ────────────────────────────────────────────────────────────
#section("Zusammenfassung")
#block(stroke: 0.8pt + line-color, radius: 5pt, inset: 10pt, width: 100%)[
  #set text(size: 9.5pt)
  #grid(
    columns: (1fr, auto),
    row-gutter: 7pt,
    text(fill: muted, "Angesetzte Tage"), str(t.days) + " von höchstens " + str(r.dayLimit),
    text(fill: muted, "Summe der angesetzten Belege"), money(t.receipts),
    text(fill: muted, "davon steuerfrei"), money(t.taxFree),
    text(fill: muted, "davon steuerpflichtig (Sachbezugswert)"), money(t.taxable),
    grid.cell(colspan: 2, line(length: 100%, stroke: 0.5pt + line-color)),
    text(weight: "bold", size: 11pt, "Auszahlung an den Mitarbeiter"), text(weight: "bold", size: 11pt, fill: accent, money(t.allowance)),
  )
]
#v(4pt)
#block(fill: soft, radius: 5pt, inset: 10pt, width: 100%)[
  #set text(size: 8.8pt)
  *Versteuerung:* #model-label.at(r.model) \
  #if r.model == "A" [
    #v(2pt)
    #grid(
      columns: (1fr, auto),
      row-gutter: 5pt,
      [Pauschale Lohnsteuer 25 % auf #money(t.taxable) (§ 40 Abs. 2 S. 1 Nr. 1 EStG)], money(t.flatTax),
      [Solidaritätszuschlag 5,5 %], money(t.soli),
      [Kirchensteuer #if s.company.churchTaxBp == 0 [(Nachweis Nichtmitgliedschaft)] else [#pct(s.company.churchTaxBp)]], money(t.churchTax),
      text(weight: "semibold", "Vom Arbeitgeber abzuführen"), text(weight: "semibold", money(t.companyTax)),
    )
    Sozialversicherungsfrei wegen Pauschalversteuerung.
  ] else [
    Steuerpflichtiger Anteil #money(t.taxable) ist über die Lohnabrechnung zu versteuern.
  ]
]
#if r.values != none {
  v(4pt)
  text(size: 7.6pt, fill: muted)[
    Werte #r.values.year: Sachbezugswert Mittagessen #money(r.values.lunch), steuerfreier Zuschuss #money(r.values.topUp), Höchstbetrag je Arbeitstag #money(r.values.lunch + r.values.topUp). Quelle: #r.values.source. Ein Zuschuss je Arbeitstag, höchstens in Höhe des Belegs (R 8.1 Abs. 7 Nr. 4 LStR).
  ]
}

// ── Days ───────────────────────────────────────────────────────────────
#section("Tage")
#data-table(
  columns: (auto, auto, auto, 1fr, auto, auto, auto, auto),
  align-right: (4, 5, 6, 7),
  header: ("Datum", "Beleg", "Händler", "Hinweis", "Beleg", "Zuschuss", "steuerfrei", "steuerpfl."),
  rows: r.days.map(d => {
    let first = receipt(d.receiptIds.at(0))
    let notes = d.warnings.map(w => warning-label.at(w, default: w))
    if d.combined { notes.push("zusammengefasst") }
    if d.at("holiday", default: "") != "" { notes = notes.map(n => if n == "Feiertag" { d.holiday } else { n }) }
    let numbers = d.receiptIds.map(id => receipt(id).number).join(", ")
    let style(x) = if d.counted { x } else { text(fill: muted, x) }
    (
      style(weekday(d.date) + " " + date(d.date)),
      style(numbers),
      style(first.merchant),
      style(text(size: 7.6pt, notes.join(", "))),
      style(money(d.total)),
      style(if d.counted { money(d.allowance) } else { "–" }),
      style(if d.counted { money(d.taxFree) } else { "–" }),
      style(if d.counted { money(d.taxable) } else { "–" }),
    )
  }),
  footer: ("", "", "", "Summe", money(t.receipts), money(t.allowance), money(t.taxFree), money(t.taxable)),
)

// ── Receipts ───────────────────────────────────────────────────────────
#section("Belege")
#data-table(
  columns: (auto, auto, 1fr, auto, auto),
  align-right: (3,),
  header: ("Nr.", "Datum", "Händler / Datei", "Betrag", "SHA-256"),
  rows: s.receipts.map(x => (
    x.number,
    date(x.date),
    [#x.merchant #text(size: 7pt, fill: muted)[\ #x.filename]],
    money(x.total),
    text(size: 6.5pt, font: "Inter", fill: muted, x.sha256.slice(0, 16) + "…"),
  )),
)
#text(size: 7.6pt, fill: muted)[Belege, die nicht angesetzt wurden, sind aufgeführt, aber nicht angehängt.]

#v(18pt)
#grid(
  columns: (1fr, 1fr),
  column-gutter: 24pt,
  [#line(length: 100%, stroke: 0.5pt) #text(size: 7.6pt, fill: muted)[Ort, Datum, Unterschrift Mitarbeiter]],
  [#line(length: 100%, stroke: 0.5pt) #text(size: 7.6pt, fill: muted)[Ort, Datum, Unterschrift Arbeitgeber]],
)

// ── Appendix ───────────────────────────────────────────────────────────
#for x in s.receipts.filter(x => x.at("file", default: none) != none) {
  let pages = if x.mime == "application/pdf" { range(1, x.pages + 1) } else { (1,) }
  for p in pages {
    pagebreak()
    block(below: 6pt)[
      #text(size: 11pt, weight: "semibold", fill: accent, x.number) #h(6pt)
      #text(fill: muted)[#date(x.date) · #x.merchant · #money(x.total)#if pages.len() > 1 [ · Seite #p / #pages.len()]]
    ]
    // Photos of till receipts are narrow; a full-width page would blow them up.
    let width = if x.mime == "application/pdf" { 100% } else { 110mm }
    align(center, block(height: 1fr, width: width, if x.mime == "application/pdf" {
      image(x.file, page: p, width: 100%, height: 100%, fit: "contain")
    } else {
      image(x.file, width: 100%, height: 100%, fit: "contain")
    }))
  }
}
