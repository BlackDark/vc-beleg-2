import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { toast } from "sonner";
import {
  getSettingsOptions,
  listRatesOptions,
  saveSettingsMutation,
} from "@/api/gen/@tanstack/react-query.gen";
import type { Settings } from "@/api/gen/types.gen";
import { Money, taxModelText } from "@/components/common";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Field, Input, Select, Textarea } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/misc";
import { useSession } from "@/lib/session";
import { m } from "@/paraglide/messages";

const states: [string, string][] = [
  ["BW", "Baden-Württemberg"],
  ["BY", "Bayern"],
  ["BE", "Berlin"],
  ["BB", "Brandenburg"],
  ["HB", "Bremen"],
  ["HH", "Hamburg"],
  ["HE", "Hessen"],
  ["MV", "Mecklenburg-Vorpommern"],
  ["NI", "Niedersachsen"],
  ["NW", "Nordrhein-Westfalen"],
  ["RP", "Rheinland-Pfalz"],
  ["SL", "Saarland"],
  ["SN", "Sachsen"],
  ["ST", "Sachsen-Anhalt"],
  ["SH", "Schleswig-Holstein"],
  ["TH", "Thüringen"],
];

export function CompanyCard() {
  const qc = useQueryClient();
  const q = useQuery(getSettingsOptions());
  const [form, setForm] = useState<Settings | null>(null);
  useEffect(() => {
    if (q.data && !form) setForm(q.data);
  }, [q.data, form]);
  const save = useMutation({
    ...saveSettingsMutation(),
    onSuccess: (data) => {
      qc.setQueryData(getSettingsOptions().queryKey, data);
      qc.invalidateQueries({ queryKey: [{ _id: "getMonth" }] });
      toast.success(m.saved());
    },
  });
  if (!form) return <Skeleton className="h-96" />;
  const set = <K extends keyof Settings>(k: K, v: Settings[K]) => setForm({ ...form, [k]: v });
  const churchProof = form.churchTaxBp === 0;
  return (
    <Card>
      <CardHeader>
        <CardTitle>{m.settings_company()}</CardTitle>
        <CardDescription>{m.settings_company_text()}</CardDescription>
      </CardHeader>
      <CardContent>
        <form
          className="grid gap-4"
          onSubmit={(e) => {
            e.preventDefault();
            save.mutate({ body: form });
          }}
        >
          <Field label={m.company_name()} htmlFor="company">
            <Input
              id="company"
              value={form.companyName}
              onChange={(e) => set("companyName", e.target.value)}
            />
          </Field>
          <Field label={m.company_address()} htmlFor="address">
            <Textarea
              id="address"
              rows={2}
              value={form.companyAddress}
              onChange={(e) => set("companyAddress", e.target.value)}
            />
          </Field>
          <Field label={m.state()} htmlFor="state" hint={m.state_hint()}>
            <Select
              id="state"
              value={form.state}
              onChange={(e) => set("state", e.target.value as Settings["state"])}
            >
              {states.map(([code, name]) => (
                <option key={code} value={code}>
                  {name}
                </option>
              ))}
            </Select>
          </Field>
          <Field label={m.employee_name()} htmlFor="employee">
            <Input
              id="employee"
              value={form.employeeName}
              onChange={(e) => set("employeeName", e.target.value)}
              required
            />
          </Field>
          <Field label={m.tax_model()} htmlFor="model" hint={m.tax_model_hint()}>
            <Select
              id="model"
              value={form.taxModel}
              onChange={(e) => set("taxModel", e.target.value as Settings["taxModel"])}
            >
              {(["A", "B", "C"] as const).map((x) => (
                <option key={x} value={x}>
                  {taxModelText(x)}
                </option>
              ))}
            </Select>
          </Field>
          <Field label={m.church_tax()} htmlFor="church">
            <Select
              id="church"
              value={churchProof ? "proof" : "simplified"}
              onChange={(e) => set("churchTaxBp", e.target.value === "proof" ? 0 : 550)}
            >
              <option value="proof">{m.church_tax_proof()}</option>
              <option value="simplified">{m.church_tax_simplified()}</option>
            </Select>
          </Field>
          {!churchProof ? (
            <Field label={m.church_tax_rate()} htmlFor="churchRate" hint={m.church_tax_rate_hint()}>
              <Input
                id="churchRate"
                type="number"
                step="0.01"
                min="0.01"
                max="10"
                value={form.churchTaxBp / 100}
                onChange={(e) => set("churchTaxBp", Math.round(Number(e.target.value) * 100))}
              />
            </Field>
          ) : null}
          <Field label={m.legal_basis()} htmlFor="legal" hint={m.legal_basis_hint()}>
            <Input id="legal" value={form.legalBasis} onChange={(e) => set("legalBasis", e.target.value)} />
          </Field>
          <Button type="submit" disabled={save.isPending}>
            {m.save()}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}

export function RatesCard() {
  const q = useQuery(listRatesOptions());
  return (
    <Card>
      <CardHeader>
        <CardTitle>{m.settings_rates()}</CardTitle>
        <CardDescription>{m.settings_rates_text()}</CardDescription>
      </CardHeader>
      <CardContent className="grid gap-3 text-sm">
        {(q.data ?? []).map((v) => (
          <div key={v.year} className="grid gap-1">
            <p className="font-medium">{v.year}</p>
            <p className="text-muted-foreground">
              {m.rate_line()} <Money cents={v.lunch} /> + <Money cents={v.topUp} /> ={" "}
              <Money cents={v.lunch + v.topUp} strong />
            </p>
            <p className="text-xs text-muted-foreground">{v.source}</p>
          </div>
        ))}
        <p className="text-xs text-muted-foreground">
          {m.version({ version: useSession().data?.version ?? "" })}
        </p>
      </CardContent>
    </Card>
  );
}
