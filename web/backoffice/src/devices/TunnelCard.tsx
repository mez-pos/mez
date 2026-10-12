// Ordering from phones (docs/design/screens/backoffice-devices.md): whether a
// guest's phone can reach this venue, as a chip with a word, a sentence saying
// what that means, where the venue stands with its relay when the relay has
// said (ADR-316), and when it tries next. It changes nothing; the link is
// set in the venue's settings file.

import { Button, StatusChip } from "../../../shared/ui";
import { Panel } from "../dashboard/Panel";
import { actionProblemOf } from "./devices-logic";
import { cloudLine, detailText, retryText, tunnelLine } from "./tunnel-logic";
import { useTunnel } from "./use-tunnel";

export function TunnelCard() {
  const tunnel = useTunnel();
  const data = tunnel.data;
  const retry = data ? retryText(data, tunnel.readAt) : null;
  const detail = data ? detailText(data) : null;
  const cloud = data ? cloudLine(data.cloud, tunnel.readAt) : null;

  return (
    <Panel title="Ordering from phones" aside={data && <StatusChip status={{ kind: "tunnel", status: data.state }} />}>
      {tunnel.loading && <p className="text-base text-fg-muted">Loading</p>}
      {!tunnel.loading && tunnel.error !== null && data === undefined && (
        <div className="flex flex-wrap items-center justify-between gap-3">
          <p className="text-base text-fg-2">{actionProblemOf(tunnel.error)}</p>
          <Button variant="secondary" onClick={tunnel.refetch}>
            Try again
          </Button>
        </div>
      )}
      {data && (
        <div className="flex flex-col gap-1.5" aria-live="polite">
          <p className="max-w-prose text-base text-fg-2">{tunnelLine(data, tunnel.readAt)}</p>
          {cloud && <p className="max-w-prose text-base text-fg-2">{cloud}</p>}
          {retry && <p className="text-base font-semibold">{retry}</p>}
          {detail && <p className="text-sm wrap-break-word text-fg-muted">What it said: {detail}</p>}
        </div>
      )}
    </Panel>
  );
}
