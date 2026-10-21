// Display stored capture timestamps in local time, always with three millisecond digits.
export function formatTimestamp(value: string, withDate = false): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "—";
  const time = new Intl.DateTimeFormat("fr-BE", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hourCycle: "h23",
  }).format(date);
  const precise = `${time}.${String(date.getMilliseconds()).padStart(3, "0")}`;
  return withDate
    ? `${new Intl.DateTimeFormat("fr-BE").format(date)} ${precise}`
    : precise;
}
