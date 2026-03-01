import { create } from 'zustand';
import { persist } from 'zustand/middleware';

// Types
export interface User {
  id: string;
  name: string;
  avatar: string;
  role: string;
  tenantId: string;
}

export interface Alert {
  id: string;
  title: string;
  source: string;
  sev: "Critical" | "High" | "Medium" | "Low";
  time: string;
  tags: string[];
  owner?: string;
  status: "New" | "Triaged" | "Closed";
  tenantId: string;
}

export interface Case {
  id: string;
  title: string;
  sev: "Critical" | "High" | "Medium" | "Low";
  status: "Open" | "Investigating" | "Contained" | "Closed";
  owner?: string;
  time: string;
  tags: string[];
  forumId?: string;
  tenantId: string;
  description?: string; // Added for forum detail view
}

export interface ForumPost {
  id: string;
  threadId: string;
  authorId: string;
  content: string;
  timestamp: string;
}

export interface ForumThread {
  id: string;
  caseId: string;
  title: string;
  status: "In Progress" | "Closed" | "Questions";
  posts: ForumPost[];
  tenantId: string;
}

export interface ObservableType {
  id: string;
  name: string;
  validatorRegex?: string;
  dataType: "string" | "number" | "boolean" | "json";
  tenantId: string; // Observables can be tenant-specific or global (if tenantId === 'global')
}

export interface Template {
  id: string;
  name: string;
  content: string;
  type: "responder" | "analyzer";
  tenantId: string;
}

export interface CaseTemplate {
  id: string;
  name: string;
  description: string;
  severity: "Critical" | "High" | "Medium" | "Low";
  tags: string[];
  tasks: string[];
  tenantId: string;
}

export interface AppState {
  currentTenantId: string;
  currentUser: User;
  alerts: Alert[];
  cases: Case[];
  forumThreads: ForumThread[];
  observableTypes: ObservableType[];
  templates: Template[];
  caseTemplates: CaseTemplate[];
  
  // Actions
  setCurrentTenant: (tenantId: string) => void;
  updateUserAvatar: (avatar: string) => void;
  assignAlert: (alertId: string, userId: string) => void;
  assignCase: (caseId: string, userId: string) => void;
  createForumThread: (caseId: string, title: string) => void;
  addForumPost: (threadId: string, content: string) => void;
  addTemplate: (template: Template) => void;
  addObservableType: (type: ObservableType) => void;
  addCaseTemplate: (template: CaseTemplate) => void;
}

// Initial Data
const initialAlerts: Alert[] = [
  // Main Corp Alerts
  { id: "AL-7712", title: "Impossible travel sign-in", source: "Okta", sev: "High", time: "2026-02-12T10:30:00", tags: ["Identity", "Auth"], status: "New", tenantId: "tenant_1" },
  { id: "AL-7701", title: "New mailbox rule created", source: "M365", sev: "Critical", time: "2026-02-12T09:15:00", tags: ["Email", "Persistence"], status: "New", tenantId: "tenant_1" },
  // Fintech Sub Alerts
  { id: "AL-7688", title: "Suspicious handle to LSASS", source: "CrowdStrike", sev: "High", time: "2026-02-11T14:20:00", tags: ["Endpoint", "Credential"], owner: "analyst_1", status: "Triaged", tenantId: "tenant_2" },
  { id: "AL-7674", title: "Large DNS TXT query volume", source: "CoreDNS", sev: "Medium", time: "2026-02-10T16:45:00", tags: ["Network", "Exfiltration"], status: "New", tenantId: "tenant_2" },
];

const initialCases: Case[] = [
  // Main Corp Cases
  { id: "INC-1249", title: "Suspicious OAuth consent spike", sev: "Critical", status: "Investigating", owner: "analyst_1", time: "2026-02-12T11:00:00", tags: ["OAuth", "Identity"], forumId: "thread_1", tenantId: "tenant_1", description: "A sudden spike in OAuth consent grants was detected for an unverified application 'Unknown App' (Client ID: 8372...9283). The consents request 'Mail.Read' and 'User.Read' scopes. Impacted users include executive leadership." },
  // Fintech Sub Cases
  { id: "INC-1244", title: "Credential dumping attempt", sev: "High", status: "Contained", owner: "analyst_2", time: "2026-02-11T15:30:00", tags: ["Persistence", "Endpoint"], tenantId: "tenant_2", description: "Endpoint Detection & Response (EDR) blocked a process attempting to dump LSASS memory on host 'FIN-WRK-002'. Preliminary analysis suggests use of a modified Mimikatz variant." },
  { id: "INC-1238", title: "New phishing kit domain", sev: "Medium", status: "Open", time: "2026-02-10T09:00:00", tags: ["Phishing", "Network"], tenantId: "tenant_2", description: "Threat Intelligence feeds identified a newly registered domain 'secure-login-update.com' hosting a phishing kit targeting our customer portal." },
];

const initialForumThreads: ForumThread[] = [
  {
    id: "thread_1",
    caseId: "INC-1249",
    title: "Discussion: Suspicious OAuth consent spike",
    status: "In Progress",
    posts: [
      { id: "post_1", threadId: "thread_1", authorId: "analyst_1", content: "I noticed a spike in OAuth consents for the 'Unknown App' yesterday. Investigating logs.", timestamp: "2026-02-12T11:05:00" },
      { id: "post_2", threadId: "thread_1", authorId: "analyst_2", content: "Check the IP reputation of the consent requests.", timestamp: "2026-02-12T11:10:00" }
    ],
    tenantId: "tenant_1"
  }
];

const initialObservableTypes: ObservableType[] = [
  { id: "ip", name: "IP Address", dataType: "string", validatorRegex: "^(?:[0-9]{1,3}\\.){3}[0-9]{1,3}$", tenantId: "global" },
  { id: "domain", name: "Domain Name", dataType: "string", validatorRegex: "^([a-z0-9]+(-[a-z0-9]+)*\\.)+[a-z]{2,}$", tenantId: "global" },
  { id: "hash_md5", name: "MD5 Hash", dataType: "string", validatorRegex: "^[a-f0-9]{32}$", tenantId: "global" },
  { id: "hash_sha256", name: "SHA256 Hash", dataType: "string", validatorRegex: "^[a-f0-9]{64}$", tenantId: "global" },
  { id: "email", name: "Email Address", dataType: "string", validatorRegex: "^[\\w-\\.]+@([\\w-]+\\.)+[\\w-]{2,4}$", tenantId: "global" },
  { id: "user_agent", name: "User Agent", dataType: "string", tenantId: "global" },
];

const initialTemplates: Template[] = [
  { id: "tpl_1", name: "VirusTotal IP Report", type: "analyzer", content: "<div><h3>VirusTotal Report</h3><p>Score: {{score}}</p><p>Country: {{country}}</p></div>", tenantId: "global" },
  { id: "tpl_2", name: "Block IP Action", type: "responder", content: "<div><h3>Action Executed</h3><p>Blocked IP {{ip}} on Firewall {{firewall_id}}</p></div>", tenantId: "global" }
];

const initialCaseTemplates: CaseTemplate[] = [
  { 
    id: "ct_phishing", 
    name: "Phishing Investigation", 
    description: "Standard procedure for reported phishing emails",
    severity: "High",
    tags: ["Phishing", "Email"],
    tasks: ["Analyze headers", "Check sender reputation", "Scan attachments", "Search for similar subjects"],
    tenantId: "global"
  },
  { 
    id: "ct_malware", 
    name: "Malware Infection", 
    description: "Response to endpoint malware detection",
    severity: "Critical",
    tags: ["Malware", "Endpoint"],
    tasks: ["Isolate host", "Collect memory dump", "Scan filesystem", "Identify entry vector"],
    tenantId: "global"
  }
];

export const useStore = create<AppState>()(
  persist(
    (set) => ({
      currentTenantId: "tenant_1",
      currentUser: {
        id: "analyst_1",
        name: "A. Rivera",
        avatar: "AR",
        role: "L3 Lead",
        tenantId: "tenant_1"
      },
      alerts: initialAlerts,
      cases: initialCases,
      forumThreads: initialForumThreads,
      observableTypes: initialObservableTypes,
      templates: initialTemplates,
      caseTemplates: initialCaseTemplates,

      setCurrentTenant: (tenantId) => set({ currentTenantId: tenantId }),
      updateUserAvatar: (avatar) => set((state) => ({ currentUser: { ...state.currentUser, avatar } })),
      
      assignAlert: (alertId, userId) => set((state) => ({
        alerts: state.alerts.map(a => a.id === alertId ? { ...a, owner: userId, status: "Triaged" } : a)
      })),
      
      assignCase: (caseId, userId) => set((state) => ({
        cases: state.cases.map(c => c.id === caseId ? { ...c, owner: userId, status: "Investigating" } : c)
      })),

      createForumThread: (caseId, title) => set((state) => {
        // Find the case to get the tenantId
        const linkedCase = state.cases.find(c => c.id === caseId);
        const tenantId = linkedCase ? linkedCase.tenantId : state.currentTenantId;

        const newThread: ForumThread = {
          id: `thread_${Date.now()}`,
          caseId,
          title,
          status: "In Progress",
          posts: [],
          tenantId
        };
        // Also link case to thread
        const updatedCases = state.cases.map(c => c.id === caseId ? { ...c, forumId: newThread.id } : c);
        return {
          forumThreads: [...state.forumThreads, newThread],
          cases: updatedCases
        };
      }),

      addForumPost: (threadId, content) => set((state) => ({
        forumThreads: state.forumThreads.map(t => t.id === threadId ? {
          ...t,
          posts: [...t.posts, {
            id: `post_${Date.now()}`,
            threadId,
            authorId: state.currentUser.id,
            content,
            timestamp: new Date().toISOString()
          }]
        } : t)
      })),

      addTemplate: (template) => set((state) => ({ templates: [...state.templates, { ...template, tenantId: state.currentTenantId }] })),
      addObservableType: (type) => set((state) => ({ observableTypes: [...state.observableTypes, { ...type, tenantId: state.currentTenantId }] })),
      addCaseTemplate: (template) => set((state) => ({ caseTemplates: [...state.caseTemplates, { ...template, tenantId: state.currentTenantId }] }))
    }),
    {
      name: 'soc-platform-storage',
    }
  )
);
