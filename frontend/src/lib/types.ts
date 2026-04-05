export type UserRole = "admin" | "university";

export interface Session {
  access_token: string;
  token_type: string;
  expires_at: string;
  role: UserRole;
  status: string;
  vuz_id?: string;
  email: string;
}

export interface AdminStats {
  total_universities: number;
  pending_universities: number;
  active_universities: number;
  blocked_universities: number;
  total_batches: number;
  total_diplomas: number;
  revoked_diplomas: number;
}

export interface UniversityRecord {
  id: string;
  vuz_code: string;
  name: string;
  inn: string;
  ogrn: string;
  email: string;
  status: string;
  has_public_key: boolean;
  created_at: string;
}

export interface SigningKeyStatus {
  configured: boolean;
  key_algorithm: string;
  encryption_algorithm: string;
  public_key_fingerprint: string;
  updated_at: string;
}

export interface Batch {
  id: string;
  vuz_id: string;
  status: string;
  total_records: number;
  processed_records: number;
  failed_records: number;
  created_at: string;
  completed_at?: string | null;
}

export interface ApiKeySummary {
  id: string;
  name?: string | null;
  is_active: boolean;
  created_at: string;
}

export interface ApiKeyCreateResponse {
  id: string;
  name?: string | null;
  api_key: string;
  created_at: string;
}

export interface BatchUploadResponse {
  batch_id: string;
  status: string;
}

export interface StudentSearchResult {
  diploma_hash: string;
  diploma_number: string;
  full_name: string;
  specialty: string;
  degree: string;
  faculty: string;
  year: number;
  university_id: string;
  university_name: string;
  status: string;
  created_at: string;
}

export interface ShareLinkResponse {
  share_url: string;
  token: string;
  expires_at: string;
}

export interface SharedDiplomaResponse {
  diploma_hash: string;
  diploma_number: string;
  full_name: string;
  specialty: string;
  degree: string;
  faculty: string;
  year: number;
  university_id: string;
  university_name: string;
  status: string;
  expires_at: string;
}

export interface VerificationByNumberResponse {
  valid: boolean;
  status: string;
  university?: string;
  vuz_code?: string;
  year?: number | null;
  specialty?: string | null;
  degree?: string | null;
  faculty?: string | null;
}

export interface VerificationByPayloadResponse {
  valid: boolean;
  status: string;
  hash?: string;
  diploma_number?: string;
  university?: string;
  vuz_code?: string;
  year?: number | null;
  specialty?: string | null;
  degree?: string | null;
  faculty?: string | null;
  revoked_at?: string | null;
}

export interface VerificationStatusCount {
  status: string;
  count: number;
}

export interface VerificationTimeBucket {
  date: string;
  count: number;
}

export interface VerificationGeoPoint {
  country?: string;
  city?: string;
  count: number;
}

export interface VerificationTopUniversity {
  vuz_id?: string;
  vuz_code?: string;
  name?: string;
  checks: number;
}

export interface VerificationStatsResponse {
  from: string;
  to: string;
  total_checks: number;
  unique_requesters: number;
  statuses: VerificationStatusCount[];
  timeseries: VerificationTimeBucket[];
  geography: VerificationGeoPoint[];
  top_universities?: VerificationTopUniversity[];
}

export interface DiplomaRecordInput {
  full_name: string;
  diploma_number: string;
  specialty: string;
  degree: string;
  faculty: string;
  year: number;
}
