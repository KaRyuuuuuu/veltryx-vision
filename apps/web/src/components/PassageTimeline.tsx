import { useEffect, useMemo, useRef, useState } from "react";
import { Passage } from "../services/api";
import { formatTimestamp } from "../services/time";

export default function PassageTimeline({ passages }: { passages: Passage[] }) {
  const chronological = useMemo(() => [...passages].reverse(), [passages]);
  const [selected, setSelected] = useState<string | null>(null);
  const track = useRef<HTMLDivElement>(null);
  const found = chronological.findIndex((p) => p.id === selected);
  const index = found >= 0 ? found : chronological.length - 1;
  const current = chronological[index];

  function select(id: string, behavior: ScrollBehavior = "smooth") {
    setSelected(id);
    const container = track.current;
    const card = container?.querySelector<HTMLElement>(
      `[data-passage="${id}"]`,
    );
    if (container && card) {
      const left = Math.max(
        0,
        Math.min(
          container.scrollWidth - container.clientWidth,
          card.offsetLeft - (container.clientWidth - card.offsetWidth) / 2,
        ),
      );
      container.scrollTo({ left, behavior });
    }
  }

  useEffect(() => {
    if (current && (selected === null || found < 0)) select(current.id, "auto");
  }, [current?.id, found, selected]);

  function move(offset: number) {
    const next = chronological[index + offset];
    if (next) select(next.id);
  }

  function scrolled() {
    const container = track.current;
    if (!container) return;
    if (container.scrollLeft <= 1) {
      setSelected(chronological[0].id);
      return;
    }
    if (
      container.scrollLeft + container.clientWidth >=
      container.scrollWidth - 1
    ) {
      setSelected(chronological[chronological.length - 1].id);
      return;
    }
    const center = container.scrollLeft + container.clientWidth / 2;
    let closest: HTMLElement | null = null;
    let distance = Infinity;
    container
      .querySelectorAll<HTMLElement>("[data-passage]")
      .forEach((card) => {
        const delta = Math.abs(card.offsetLeft + card.offsetWidth / 2 - center);
        if (delta < distance) {
          distance = delta;
          closest = card;
        }
      });
    const id = (closest as HTMLElement | null)?.dataset.passage;
    if (id) setSelected(id);
  }

  if (!current)
    return (
      <div className="empty">
        Les captures manuelles et les passages détectés apparaîtront ici.
      </div>
    );

  return (
    <section className="passage-timeline" aria-label="Timeline des passages">
      <div className="timeline-toolbar">
        <div>
          <h2>Timeline des passages</h2>
          <p className="muted">
            Une image par passage · glissez horizontalement ou utilisez les
            flèches.
          </p>
        </div>
        <div className="timeline-navigation">
          <button
            onClick={() => move(-1)}
            disabled={index <= 0}
            aria-label="Passage précédent"
          >
            ←
          </button>
          <span aria-live="polite">
            {index + 1} / {chronological.length}
          </span>
          <button
            onClick={() => move(1)}
            disabled={index >= chronological.length - 1}
            aria-label="Passage suivant"
          >
            →
          </button>
          <button
            onClick={() => select(chronological[chronological.length - 1].id)}
          >
            Dernier passage
          </button>
        </div>
      </div>
      <div
        className="timeline-track"
        ref={track}
        onScroll={scrolled}
        tabIndex={0}
        role="region"
        aria-label="Passages par ordre chronologique. Flèches gauche et droite pour naviguer."
        onKeyDown={(event) => {
          if (event.key === "ArrowLeft" || event.key === "ArrowRight") {
            event.preventDefault();
            move(event.key === "ArrowLeft" ? -1 : 1);
          }
          if (event.key === "Home") {
            event.preventDefault();
            select(chronological[0].id);
          }
          if (event.key === "End") {
            event.preventDefault();
            select(chronological[chronological.length - 1].id);
          }
        }}
      >
        {chronological.map((p) => (
          <article
            key={p.id}
            data-passage={p.id}
            className={`timeline-card ${p.id === current.id ? "selected" : ""}`}
          >
            <button
              className="timeline-time"
              onClick={() => select(p.id)}
              aria-pressed={p.id === current.id}
            >
              <time dateTime={p.timestamp}>
                {formatTimestamp(p.timestamp, true)}
              </time>
            </button>
            <a
              className="timeline-image"
              href={`/api/v1/screenshots/${p.representative.id}/image`}
              target="_blank"
              rel="noreferrer"
              onFocus={() => setSelected(p.id)}
              title="Ouvrir la capture originale"
            >
              <img
                loading="lazy"
                src={`/api/v1/screenshots/${p.representative.id}/image`}
                alt={`Passage ${p.serial} à ${formatTimestamp(p.timestamp, true)}`}
              />
            </a>
            <div className="timeline-caption">
              <strong>
                {p.serial} · {p.zone ? "Détection ROI" : "Capture manuelle"}
              </strong>
              <span>
                {p.representative.role === "trigger"
                  ? "Image au déclenchement"
                  : p.image_count > 1
                    ? "Image centrale du passage"
                    : "Capture unique"}
              </span>
              <span className="muted">
                {p.image_count} image(s) conservée(s) · OCR non configuré
              </span>
            </div>
          </article>
        ))}
      </div>
    </section>
  );
}
