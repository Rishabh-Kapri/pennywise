import { Injectable, inject } from '@angular/core';
import {
  Firestore,
  QuerySnapshot,
  and,
  collectionData,
  doc,
  getDocs,
  query,
  updateDoc,
  where,
  writeBatch,
} from '@angular/fire/firestore';
import { addDoc, collection } from 'firebase/firestore';
import { Observable } from 'rxjs';
import { Account } from '../models/account.model';
import { Budget } from '../models/budget.model';
import { Category, CategoryDTO, InflowCategory } from '../models/category.model';
import { CategoryGroup } from '../models/catergoryGroup';
import { Payee } from '../models/payee.model';
import { Transaction } from '../models/transaction.model';
import { INFLOW_CATEGORY_NAME, MASTER_CATEGORY_GROUP_NAME } from '../constants/general';
import { HelperService } from './helper.service';

@Injectable({
  providedIn: 'root',
})
export class DatabaseService {
  firestore: Firestore = inject(Firestore);

  accountCollection = collection(this.firestore, 'accounts');
  budgetCollection = collection(this.firestore, 'budgets');
  categoryGroupCollection = collection(this.firestore, 'categoryGroups');
  categoryCollection = collection(this.firestore, 'categories');
  payeeCollection = collection(this.firestore, 'payees');
  transactionCollection = collection(this.firestore, 'transactions');

  constructor(private helperService: HelperService) {}

  async createPayee(payee: Payee) {
    return await addDoc(this.payeeCollection, payee);
  }

  dummyPromise() {
    return new Promise((resolve) => {
      setTimeout(() => {
        resolve(true);
      }, 1000);
    });
  }

  async assignZeroToUnassignedCategories(categories: Category[], selectedMonth: string) {
    const batch = writeBatch(this.firestore);

    categories.forEach((category) => {
      if (category.name !== INFLOW_CATEGORY_NAME && category.budgeted?.[selectedMonth] === undefined) {
        console.log('category:', category, selectedMonth);
        const catRef = doc(this.categoryCollection, category.id);
        batch.update(catRef, {
          budgeted: {
            ...category.budgeted,
            [selectedMonth]: 0,
          },
        });
      }
    });
    console.log(batch);
    await batch.commit();
  }

  async createBudget(budget: Budget) {
    const createdBudget = await addDoc(this.budgetCollection, budget);
    const masterCategoryGroup: CategoryGroup = {
      budgetId: createdBudget.id,
      name: MASTER_CATEGORY_GROUP_NAME,
      hidden: false,
      deleted: false,
      createdAt: new Date().toISOString(),
    };
    const createdGroup = await this.createCategoryGroup(masterCategoryGroup);
    const inflowCategory: CategoryDTO = {
      budgetId: createdBudget.id,
      categoryGroupId: createdGroup.id,
      name: INFLOW_CATEGORY_NAME,
      budgeted: 0,
      deleted: false,
      createdAt: new Date().toISOString(),
    };
    await this.createCategory(inflowCategory);
    return createdBudget;
  }

  // only allows for editing isSelected and name for now
  async editBudget(budget: Budget) {
    const budgetRef = doc(this.budgetCollection, budget.id);
    await updateDoc(budgetRef, {
      ...budget,
      updatedAt: new Date().toISOString(),
    });
  }

  async createAccount(account: Account) {
    // create account
    const createdAccount = await addDoc(this.accountCollection, account);
    const payee: Payee = {
      budgetId: account.budgetId,
      name: `Transfer - ${account.name}`,
      transferAccountId: createdAccount.id,
      createdAt: new Date().toISOString(),
      deleted: false,
    };
    // create payee
    const createdPayee = await this.createPayee(payee);
    // update account with payeeId
    await updateDoc(createdAccount, { payeeId: createdPayee.id });
  }

  async editAccount(account: Account) {
    const accountRef = doc(this.accountCollection, account.id);
    await updateDoc(accountRef, {
      ...account,
      updatedAt: new Date().toISOString(),
    });
  }

  async createCategoryGroup(group: CategoryGroup) {
    return await addDoc(this.categoryGroupCollection, group);
  }

  async createCategory(category: CategoryDTO) {
    return await addDoc(this.categoryCollection, category);
  }

  async editCategory(category: CategoryDTO) {
    const categoryRef = doc(this.categoryCollection, category.id);
    return await updateDoc(categoryRef, {
      ...category,
      updatedAt: new Date().toISOString(),
    });
  }

  async createTransaction(transaction: Transaction) {
    return await addDoc(this.transactionCollection, transaction);
  }

  getBudgetsStream() {
    return collectionData(this.budgetCollection, { idField: 'id' }) as Observable<Budget[]>;
  }

  getSelectedBudgetStream() {
    const q = query(this.budgetCollection, where('isSelected', '==', true));
    return collectionData(q, { idField: 'id' }) as Observable<Budget[]>;
  }

  getAccountsStream(budgetId: string) {
    const q = query(this.accountCollection, where('budgetId', '==', budgetId));
    return collectionData(q, { idField: 'id' }) as Observable<Account[]>;
  }

  getCategoryGroupsStream(budgetId: string) {
    const q = query(this.categoryGroupCollection, where('budgetId', '==', budgetId));
    const data = collectionData(q, { idField: 'id' }) as Observable<CategoryGroup[]>;
    return data;
  }

  getCategoriesStream(budgetId: string) {
    const q = query(this.categoryCollection, and(where('deleted', '==', false), where('budgetId', '==', budgetId)));
    const data = collectionData(q, { idField: 'id' }) as Observable<CategoryDTO[]>;
    return data;
  }

  getMonthsTransactionsStream(monthKey: string, budgetId: string) {
    const { year, month } = this.helperService.splitKeyIntoMonthYear(monthKey);
    const q = query(
      this.transactionCollection,
      and(
        where('date', '>=', new Date(year, month, 0).toISOString()),
        where('date', '<', new Date(year, month + 1, 0).toISOString()),
        where('budgetId', '==', budgetId)
      )
    );
    const data = collectionData(q, { idField: 'id' }) as Observable<Transaction[]>;
    return data;
  }

  getAllTransactionsStream(budgetId: string) {
    const q = query(this.transactionCollection, and(where('budgetId', '==', budgetId)));
    return collectionData(q, { idField: 'id' }) as Observable<Transaction[]>;
  }

  async getMonthsTransactions(monthKey: string, budgetId: string) {
    const { year, month } = this.helperService.splitKeyIntoMonthYear(monthKey);
    console.log('getMonthsTransactions');
    console.log(new Date(year, month, 0).toISOString());
    const date = new Date(year, month, 0);
    console.log('DATE:', new Date(date.getTime() + date.getTimezoneOffset() * 60000).toISOString());
    const q = query(
      this.transactionCollection,
      and(
        where('date', '>=', new Date(year, month, 0).toISOString()),
        where('date', '<', new Date(year, month + 1, 0).toISOString()),
        where('budgetId', '==', budgetId)
      )
    );
    console.log('transaction query:', q);
    const docSnap = await getDocs(q);
    const transactions: Transaction[] = [];
    docSnap.forEach((doc) => {
      transactions.push(<Transaction>doc.data());
    });
    return transactions;
  }
}
