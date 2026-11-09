// Made-up example data for the sample analysis. No real statement is represented here.
export interface StatementEntry {
  date: string;
  time: string;
  description: string;
  points: number | null;
  amount: number;
  note: string;
  section: string;
  foreignAmount: string | null;
  credit: boolean;
}

export const sampleEntries: StatementEntry[] = [
  { date: '2026-08-03', time: '09:15', description: 'SAMPLE GROCERY STORE', points: 40, amount: 2000, note: '', section: 'Domestic', foreignAmount: null, credit: false },
  { date: '2026-08-05', time: '13:40', description: 'SAMPLE TELECOM RECHARGE', points: 10, amount: 500, note: '', section: 'Domestic', foreignAmount: null, credit: false },
  { date: '2026-08-09', time: '20:05', description: 'SAMPLE RESTAURANT', points: null, amount: 750, note: '', section: 'Domestic', foreignAmount: null, credit: false },
  { date: '2026-08-12', time: '11:30', description: 'SAMPLE DESIGN APP', points: 20, amount: 1000, note: '', section: 'Domestic', foreignAmount: null, credit: false },
  { date: '2026-08-14', time: '18:22', description: 'SAMPLE NOTES APP', points: 6, amount: 300, note: '', section: 'Domestic', foreignAmount: null, credit: false },
  { date: '2026-08-18', time: '07:50', description: 'SAMPLE AI SERVICE', points: 40, amount: 2000, note: '', section: 'International', foreignAmount: 'USD 24.00', credit: false },
  { date: '2026-08-21', time: '16:10', description: 'SAMPLE HOSTING', points: 10, amount: 500, note: '', section: 'International', foreignAmount: 'USD 6.00', credit: false },
  { date: '2026-08-27', time: '10:45', description: 'SAMPLE STORE REFUND', points: -10, amount: 500, note: 'Apparent credit', section: 'Domestic', foreignAmount: null, credit: true },
];

export const sampleCharges = [
  { name: 'Sample Design App', detail: 'DCC fee · purchase ₹1,000.00', fee: 20, gst: 3.6, evidence: 'Inferred', source: 'Fee and GST share a date with this purchase' },
  { name: 'Sample Notes App', detail: 'DCC fee · purchase ₹300.00', fee: 5, gst: 0.9, evidence: 'Inferred', source: 'Fee and GST share a date with this purchase' },
  { name: 'Reward redemption', detail: 'Processing fee', fee: 100, gst: 18, evidence: 'Reference match', source: 'Fee and GST share a statement reference' },
  { name: 'Sample AI Service', detail: 'Estimated allocation of consolidated FX markup', fee: 40, gst: 7.2, evidence: 'Inferred', source: 'Allocated by purchase amount' },
  { name: 'Sample Hosting', detail: 'Estimated allocation of consolidated FX markup', fee: 10, gst: 1.8, evidence: 'Inferred', source: 'Allocated by purchase amount' },
  { name: 'Sample VPN', detail: 'Estimated allocation of consolidated FX markup', fee: 30, gst: 5.4, evidence: 'Inferred', source: 'Allocated by purchase amount' },
  { name: 'EMI loan 1000000001', detail: 'Interest · principal shown separately', fee: 200, gst: 0, evidence: 'Reported', source: 'Reported in the EMI section' },
  { name: 'EMI loan 1000000002', detail: 'Interest · principal shown separately', fee: 100, gst: 0, evidence: 'Reported', source: 'Reported in the EMI section' },
  { name: 'Unassigned GST', detail: 'Three tax entries needing review', fee: 0, gst: 60, evidence: 'Unassigned', source: '₹10.00, ₹30.00 and ₹20.00 at stated 18%' },
];

export const samplePrograms = [
  { name: 'Retail spends bonus', points: 100 },
  { name: 'Grocery rewards', points: 50 },
  { name: 'Telecom rewards', points: 50 },
  { name: 'Insurance rewards', points: 100 },
  { name: 'Promotional bonus', points: 200 },
];

export const sampleLoans = [
  { id: '1000000001', original: 30000, principal: 5000, interest: 200, outstanding: 10000, remainingInterest: 200 },
  { id: '1000000002', original: 15000, principal: 2500, interest: 100, outstanding: 5000, remainingInterest: 100 },
];
