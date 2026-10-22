import { describe, expect, it } from "vitest";
import { centsToInput, currentMonth, formatCents, formatMonth, parseAmount, shiftMonth } from "./format";

describe("parseAmount", () => {
  it.each([
    ["7,67", 767],
    ["7.67", 767],
    ["1.234,5", 123450],
    ["12 €", 1200],
    ["0,5", 50],
  ])("%s → %d", (input, cents) => expect(parseAmount(input)).toBe(cents));

  it.each(["", "abc", "1,234", "7,678", "-3"])("rejects %s", (input) =>
    expect(parseAmount(input)).toBeNull(),
  );
});

describe("formatting", () => {
  it("formats cents and months in German", () => {
    expect(formatCents(767).replace(/\s/g, " ")).toBe("7,67 €");
    expect(formatMonth("2026-10")).toBe("Oktober 2026");
    expect(centsToInput(1205)).toBe("12,05");
    expect(centsToInput(0)).toBe("");
  });

  it("moves months across years", () => {
    expect(shiftMonth("2026-01", -1)).toBe("2025-12");
    expect(shiftMonth("2026-12", 1)).toBe("2027-01");
    expect(currentMonth(new Date(2026, 9, 22))).toBe("2026-10");
  });
});
