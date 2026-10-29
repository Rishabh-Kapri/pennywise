# Bank logos

Shown by `src/features/accounts/bankLogos.ts` for accounts with that `bank` id
(or, for older accounts without one, whose name mentions the bank, e.g. "HDFC
Checking"). `<id>-mark.svg` is the logo the UI shows; `<id>.svg` is the full
wordmark, currently unused. The mark is the same file with its `viewBox`
cropped to the symbol. No other edits were made.

| id | Source | Licence on source page |
|----|--------|------------------------|
| hdfc | https://commons.wikimedia.org/wiki/File:HDFC-Bank-Logo.svg | Public domain |
| icici | https://commons.wikimedia.org/wiki/File:ICICI_Bank_Logo.svg | Public domain |
| sbi | https://commons.wikimedia.org/wiki/File:State_Bank_of_India.svg (full), https://commons.wikimedia.org/wiki/File:SBI-logo.svg (mark) | Public domain |
| axis | https://commons.wikimedia.org/wiki/File:Axis_Bank_logo.svg | Public domain |
| pnb | https://commons.wikimedia.org/wiki/File:Punjab_National_Bank_new_logo.svg | Public domain |
| canara | https://commons.wikimedia.org/wiki/File:Canara_Bank_Logo.svg | Public domain |
| kotak | https://en.wikipedia.org/wiki/File:Kotak_Mahindra_Group_logo.svg | Public domain (the "Kotak Mahindra Bank" file is non-free, so the group mark is used) |

The logos are trademarks of their respective banks; they are used here only to
identify the user's own accounts. To add a bank, drop `<id>.svg` and
`<id>-mark.svg` here and register it in `bankLogos.ts`.
