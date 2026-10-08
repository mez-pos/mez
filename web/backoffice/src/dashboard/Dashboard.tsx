// The back office home (D12): today at a glance. Four figures along the top,
// then sales through the day beside what sold and the tables in use, then who
// sold it beside what is running low and the shortcuts.
//
// Each card is for whoever may read what is in it: the sales cards need
// reports.view, the tables order.open and the stock stock.manage. A card
// nobody could fill is not drawn.

import { ArmchairIcon, BanknoteIcon, ReceiptTextIcon, TagIcon } from "lucide-react";
import { useAuth } from "../../../shared/auth/use-auth";
import { Card, Money } from "../../../shared/ui";
import { useClock } from "../../../shared/use-clock";
import { lastDays, tradingDay } from "../audit/audit-logic";
import { averageBill } from "../sales/sales-logic";
import { usePermissions } from "../shell/BackOfficeShell";
import { useStockItems } from "../stock/use-stock";
import { ActiveTables, LowStock, StaffPerformance, TopItems } from "./DashLists";
import { change, occupancy } from "./dashboard-logic";
import { Nothing, Panel, ReadState } from "./Panel";
import { BrandCard, QuickActions, quickActions } from "./QuickActions";
import { SalesOverview } from "./SalesOverview";
import { ChangeNote, StatTile } from "./StatTile";
import { useDaySales, useFloorNow } from "./use-dashboard";

// Three across from 1280px: the wide card, then two lists. At the design's
// width the lists are its 400px.
const row = "grid grid-cols-1 gap-4 lg:grid-cols-2 xl:grid-cols-[minmax(0,3fr)_minmax(0,2fr)_minmax(0,2fr)] min-[1800px]:grid-cols-[minmax(0,1fr)_400px_400px]";
const wide = "lg:col-span-2 xl:col-span-1";

export function Dashboard() {
  const auth = useAuth();
  const held = usePermissions();
  // The minute, so the day turns over and a table's wait moves on a screen left open.
  const now = useClock(60_000);
  const today = tradingDay(new Date(now));
  const yesterday = lastDays(new Date(now), 2).from;

  const canSales = held.includes("reports.view");
  const canFloor = held.includes("order.open");
  const canStock = held.includes("stock.manage");

  const sales = useDaySales(today, canSales);
  const before = useDaySales(yesterday, canSales);
  const floor = useFloorNow(canFloor);
  const stock = useStockItems(canStock);

  if (!canSales && !canFloor && !canStock && quickActions(held).length === 0) {
    return (
      <Card padding="lg" className="max-w-prose">
        <h1 className="font-serif text-xl">Dashboard</h1>
        <p className="mt-2 text-base text-fg-2">Your role opens no back office sections. Ask the owner if you need one.</p>
      </Card>
    );
  }

  const day = sales.data?.totals.sales;
  const prior = before.data?.totals.sales;
  const average = day ? averageBill(day.total, day.bills) : null;
  const priorAverage = prior ? averageBill(prior.total, prior.bills) : null;
  const seats = floor.data ? occupancy(floor.data.tables) : null;
  const reading = <span className="font-sans text-base font-normal text-fg-muted">{sales.isError ? "Not read" : "Reading"}</span>;

  return (
    <div className="flex flex-col gap-4">
      <h1 className="sr-only">Dashboard{auth.kind === "signed_in" ? `, signed in as ${auth.staff.name}` : ""}</h1>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {canSales && (
          <StatTile
            icon={BanknoteIcon}
            tone="success"
            label="Today's sales"
            featured
            note={day && prior && <ChangeNote change={change(day.total.minor, prior.total.minor)} />}
          >
            {day ? <Money amount={day.total} variant="total" /> : reading}
          </StatTile>
        )}
        {canSales && (
          <StatTile icon={ReceiptTextIcon} tone="attention" label="Total orders" note={day && prior && <ChangeNote change={change(day.bills, prior.bills)} />}>
            {day ? day.bills : reading}
          </StatTile>
        )}
        {canFloor && (
          <StatTile
            icon={ArmchairIcon}
            tone="warning"
            label="Active tables"
            note={seats && (seats.total > 0 ? `${seats.percent}% occupied` : "No tables on the floor plan")}
          >
            {seats ? (
              `${seats.active} / ${seats.total}`
            ) : (
              <span className="font-sans text-base font-normal text-fg-muted">{floor.isError ? "Not read" : "Reading"}</span>
            )}
          </StatTile>
        )}
        {canSales && (
          <StatTile
            icon={TagIcon}
            tone="info"
            label="Average order value"
            note={average && priorAverage && <ChangeNote change={change(average.minor, priorAverage.minor)} />}
          >
            {!day ? (
              reading
            ) : average ? (
              <Money amount={average} variant="total" />
            ) : (
              <span className="font-sans text-lg font-normal text-fg-muted">No bills yet</span>
            )}
          </StatTile>
        )}
      </div>

      <div className={row}>
        {canSales && (
          <Panel title="Sales overview" more={{ label: "View reports", href: "/sales" }} className={wide}>
            {sales.data ? (
              <SalesOverview hours={sales.data.hours} currency={sales.data.totals.currency} />
            ) : (
              <ReadState what="today's sales" failed={sales.isError} onRetry={() => void sales.refetch()} />
            )}
          </Panel>
        )}
        {canSales && (
          <Panel title="Top selling items" more={{ label: "View all", href: "/sales" }}>
            {sales.data ? (
              <TopItems items={sales.data.totals.by_item} />
            ) : (
              <ReadState what="what sold" failed={sales.isError} onRetry={() => void sales.refetch()} />
            )}
          </Panel>
        )}
        {canFloor && (
          <Panel title="Active tables">
            {floor.data ? (
              <ActiveTables tables={floor.data.tables} now={now} />
            ) : (
              <ReadState what="the floor" failed={floor.isError} onRetry={() => void floor.refetch()} />
            )}
          </Panel>
        )}
      </div>

      <div className={row}>
        {canSales && (
          <Panel title="Staff performance" className={wide}>
            {sales.data ? (
              <div className="relative overflow-x-auto">
                <StaffPerformance report={sales.data} />
              </div>
            ) : (
              <ReadState what="who sold what" failed={sales.isError} onRetry={() => void sales.refetch()} />
            )}
          </Panel>
        )}
        {canStock && (
          <Panel title="Low stock alerts" more={{ label: "View all", href: "/stock" }}>
            {stock.data ? (
              <LowStock items={stock.data.items} />
            ) : stock.isPending || stock.isError ? (
              <ReadState what="the stock" failed={stock.isError} onRetry={() => void stock.refetch()} />
            ) : (
              <Nothing>Nothing is low.</Nothing>
            )}
          </Panel>
        )}
        <div className="flex min-w-0 flex-col gap-4">
          <QuickActions held={held} />
          <BrandCard />
        </div>
      </div>
    </div>
  );
}
