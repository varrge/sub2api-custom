export interface MonthCardRuleDraft {
  id: string
  title: string
  content: string
  active: boolean
}
export interface PublishedMonthCardRule {
  id: string
  title: string
  content: string
  version: string
  read_at?: string | null
}
export interface MonthCardRules {
  publication: number
  documents: PublishedMonthCardRule[]
}
export interface AdminMonthCardRules {
  draft_revision: number
  publication: number
  documents: MonthCardRuleDraft[]
  published_documents: PublishedMonthCardRule[]
  published_at: string
}
