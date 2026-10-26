import { createFileRoute, Link } from "@tanstack/react-router";
import { PageHeader } from "@/components/common";
import { ImportLogCard } from "@/components/import-log";
import { PasswordCard, ProfileCard } from "@/components/settings-account";
import { CompanyCard, RatesCard } from "@/components/settings-company";
import { FolderCard, MailCard } from "@/components/settings-integrations";
import { m } from "@/paraglide/messages";

const sections = ["company", "account", "integrations", "imports"] as const;
type Section = (typeof sections)[number];

export const Route = createFileRoute("/_app/settings")({
  validateSearch: (search: Record<string, unknown>): { section?: Section } => ({
    section: sections.includes(search.section as Section) ? (search.section as Section) : undefined,
  }),
  component: SettingsPage,
});

function SettingsPage() {
  const { section = "company" } = Route.useSearch();
  const labels = {
    company: m.settings_section_company(),
    account: m.settings_section_account(),
    integrations: m.settings_section_integrations(),
    imports: m.settings_section_imports(),
  };
  return (
    <>
      <PageHeader title={m.nav_settings()} />
      <nav aria-label={m.settings_navigation()} className="mb-4 flex flex-wrap gap-2">
        {sections.map((key) => (
          <Link
            key={key}
            to="/settings"
            search={{ section: key }}
            aria-current={section === key ? "page" : undefined}
            className={`rounded-lg border px-3 py-2 text-sm ${section === key ? "bg-primary text-primary-foreground" : "hover:bg-muted"}`}
          >
            {labels[key]}
          </Link>
        ))}
      </nav>
      <div className="grid gap-4 lg:grid-cols-2">
        {section === "company" ? (
          <>
            <CompanyCard />
            <RatesCard />
          </>
        ) : null}
        {section === "account" ? (
          <>
            <ProfileCard />
            <PasswordCard />
          </>
        ) : null}
        {section === "integrations" ? (
          <>
            <MailCard />
            <FolderCard />
          </>
        ) : null}
        {section === "imports" ? (
          <div className="lg:col-span-2">
            <ImportLogCard />
          </div>
        ) : null}
      </div>
    </>
  );
}
