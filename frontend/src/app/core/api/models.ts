// Tipos del contrato REST. Reflejan backend/openapi.yaml: si el contrato cambia
// (PR aparte), se actualizan aquí en el mismo PR.
// Montos en centavos enteros (*Cents) y cuotas en centésimas (oddX100: 214 = 2.14).

export type Role = 'user' | 'admin';
export type MatchStatus =
  | 'SCHEDULED'
  | 'LIVE'
  | 'INTERRUPTED'
  | 'FINISHED'
  | 'POSTPONED'
  | 'CANCELLED'
  | 'ABANDONED'
  | 'AWARDED';
export type MarketStatus = 'OPEN' | 'CLOSED' | 'SETTLED' | 'VOID';
export type MarketCode = 'MATCH_RESULT' | 'TOTAL_GOALS_2_5' | 'BOTH_TEAMS_TO_SCORE';
export type SelectionCode = 'HOME' | 'DRAW' | 'AWAY' | 'OVER' | 'UNDER' | 'YES' | 'NO';
export type SelectionResult = 'PENDING' | 'WON' | 'LOST' | 'VOID';
export type BetStatus = 'PENDING' | 'WON' | 'LOST' | 'VOID';
export type WalletTxType =
  | 'DEPOSIT_INITIAL'
  | 'BET_STAKE'
  | 'BET_PAYOUT'
  | 'BET_REFUND'
  | 'BET_PAYOUT_REVERSAL'
  | 'BET_REFUND_REVERSAL'
  | 'ADMIN_ADJUSTMENT';

export interface ApiErrorBody {
  error: { code: string; message: string; fields?: Record<string, string> };
}

export interface Paged<T> {
  items: T[];
  page: number;
  pageSize: number;
  total: number;
}

// ---- auth / users ----
export interface User {
  id: string;
  email: string;
  displayName: string;
  role: Role;
}
export interface UserSummary extends User {
  status: 'active' | 'blocked';
  createdAt: string;
}
export interface RegisterRequest {
  email: string;
  password: string;
  displayName: string;
}
export interface LoginRequest {
  email: string;
  password: string;
}
export interface AuthResponse {
  accessToken: string;
  expiresAt: string;
  user: User;
}

// ---- admin ----
export interface AdminStats {
  users: number;
  betsPlaced: number;
  betsSettled: number;
  stakeCents: number;
  payoutCents: number;
}
export interface AuditEntry {
  id: number;
  actorId: string | null;
  actorName?: string | null;
  action: string;
  entity: string;
  entityId: string | null;
  data?: Record<string, unknown>;
  /** En un BET_SETTLED, los datos del BET_PLACED de la misma apuesta. */
  placed?: Record<string, unknown>;
  createdAt: string;
}

// ---- events / markets ----
export interface League {
  id: string;
  name: string;
  country: string;
  featured: boolean;
  /** Logo de la liga y bandera del país (https); null en el replay. */
  logoUrl?: string | null;
  flagUrl?: string | null;
}
export interface Team {
  id: string;
  name: string;
  /** Escudo (https); null si el proveedor no lo entrega, p. ej. en el replay. */
  logoUrl?: string | null;
}
export interface SportEvent {
  id: string;
  league: League;
  home: Team;
  away: Team;
  startAt: string;
  status: MatchStatus;
  homeScore?: number | null;
  awayScore?: number | null;
  minute?: number | null;
  featured: boolean;
  liveUpdatedAt?: string | null;
  /** Jornada como la nombra el proveedor ("Regular Season - 8"). */
  round?: string | null;
  venue?: { name: string; city?: string | null } | null;
}
export interface Selection {
  id: string;
  code: SelectionCode;
  label: string;
  oddX100: number;
  result: SelectionResult;
}
export interface Market {
  id: string;
  eventId: string;
  code: MarketCode;
  status: MarketStatus;
  selections: Selection[];
}
export interface MatchEvent {
  minute: number;
  /** Descuento: 90+3 es minute 90, extra 3. */
  extra?: number | null;
  side: 'HOME' | 'AWAY';
  type: 'GOAL' | 'CARD' | 'SUBST' | 'VAR';
  /** Texto del proveedor: "Normal Goal", "Penalty", "Yellow Card"… */
  detail: string;
  player?: string | null;
  /** Asistencia; en un cambio, el otro jugador. */
  assist?: string | null;
}
export interface EventDetail extends SportEvent {
  markets: Market[];
  /** Sucesos del partido; ausente si todavía no hay. */
  timeline?: MatchEvent[];
}

// ---- betting ----
export interface PlaceBetRequest {
  stakeCents: number;
  selections: { selectionId: string; oddX100: number }[];
}
export interface BetSelection {
  selectionId: string;
  eventId: string;
  eventName: string;
  marketCode: MarketCode;
  selectionCode: SelectionCode;
  label: string;
  oddX100: number;
  result: SelectionResult;
}
export interface Bet {
  id: string;
  stakeCents: number;
  totalOddX100: number;
  potentialPayoutCents: number;
  status: BetStatus;
  payoutCents: number;
  placedAt: string;
  settledAt?: string | null;
  selections: BetSelection[];
}

// ---- wallet ----
export interface Wallet {
  balanceCents: number;
  currency: 'VIRT';
}
export interface WalletTransaction {
  id: string;
  type: WalletTxType;
  amountCents: number;
  balanceAfterCents: number;
  reference: string;
  createdAt: string;
}

// ---- SSE (/events/stream) ----
export interface MatchUpdatedMessage {
  eventId: string;
  status: MatchStatus;
  homeScore: number;
  awayScore: number;
  minute: number;
  updatedAt: string;
}
export interface MatchStartedMessage {
  eventId: string;
  startedAt: string;
}
export interface MatchFinishedMessage {
  eventId: string;
  status: MatchStatus;
  homeScore: number;
  awayScore: number;
  finishedAt: string;
}
export interface LiveMessages {
  'match.updated': MatchUpdatedMessage;
  'match.started': MatchStartedMessage;
  'match.finished': MatchFinishedMessage;
}
