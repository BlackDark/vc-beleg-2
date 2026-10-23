# Mahlzeit

Records meal receipts of an employee, computes the monthly meal allowance (Essenszuschuss) under German wage-tax rules and archives each month as a PDF report. Built for a managing shareholder (Gesellschafter-Geschäftsführer) paying the allowance to themselves.

English terms are used in code and docs; the German term is what the UI shows.

## Language

### Parties

**Company** (Firma):
The employer paying the meal allowance; it carries the Bundesland, the church-tax setting and the optional legal basis (Rechtsgrundlage) printed on reports.

**Employee** (Mitarbeiter):
The person receiving the meal allowance. Currently one per company.
_Avoid_: User (that is the login identity, not the domain person)

### Receipts

**Receipt** (Beleg):
Evidence of one meal purchase, imported by upload, mail or folder, with date, total and merchant.
_Avoid_: Bon, document, invoice

**Receipt status** (Status):
`zu prüfen` after import, `geprüft` after the employee confirmed date, total and merchant. Only checked receipts count.
_Avoid_: approved, valid

**Mail import** (Mail-Eingang):
Receipts taken from attachments of mails sent to the dedicated mailbox by an allowed sender.

**Folder import** (Ordner-Import):
Receipts taken from image and PDF files placed in a watched WebDAV folder; processed files are moved to a subfolder.

### Calculation

**Meal** (Mahlzeit):
Breakfast, lunch or dinner. The allowance is granted for one meal per working day; lunch is the default.

**Working day** (Arbeitstag):
A day the employee worked, including Homeoffice. Within the 15-day simplification it is not proven individually.

**15-day simplification** (15-Tage-Vereinfachung):
With at most 15 allowances in a month, working days need no proof.

**Benefit value** (Sachbezugswert):
The official value of a meal for a calendar year (2026 lunch: €4.57).

**Meal allowance** (Essenszuschuss):
The employer's payment for one meal on one working day, capped by the receipt total and by benefit value + €3.10.
_Avoid_: Zulage, Essensgeld, Pauschale

**Meal day** (Essenstag):
A date with at least one checked receipt. Its total is the highest receipt of that day, or the sum of all of them when the employee combines them.

**Counted day** (angesetzter Tag):
A meal day that earns an allowance in its month. Above the day limit, the meal days with the highest totals count; the rest are **not counted** (nicht angesetzt).
_Avoid_: counted receipt

**Payout** (Auszahlung):
The sum of a month's allowances, paid by the company to the employee at month end. The employee pays every meal personally first.
_Avoid_: Erstattung, Zuzahlung

**Tax model** (Versteuerungsmodell):
How the taxable part of an allowance is handled: **A** company taxes it at a flat 25 % (default), **B** taxed individually through payroll, **C** allowance limited to €3.10, fully tax-free.

**Tax-free part** (steuerfreier Anteil):
The part of an allowance above the benefit value, at most €3.10.

**Taxable part** (steuerpflichtiger Anteil):
The part of an allowance up to the benefit value.

**Flat tax** (Pauschalsteuer):
25 % wage tax on the taxable part paid by the company (§ 40 Abs. 2 EStG), plus Soli and church tax.

### Months

**Month** (Abrechnungsmonat):
One employee's calendar month: `offen`, then `abgeschlossen` (numbers frozen, report generated), optionally `ausgezahlt` with a date.

**Monthly report** (Monatsbericht):
The PDF of a closed month: summary, day table and the receipts of every counted day. Reopening a month creates a new report version; old versions stay unchanged.
