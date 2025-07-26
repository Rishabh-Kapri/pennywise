import { OverlayRef } from '@angular/cdk/overlay';
import { TemplateRef, Type, ViewContainerRef } from '@angular/core';
import { Subject } from 'rxjs';

export type PopoverContent = TemplateRef<any> | Type<any> | string;

export type PopoverParams<T> = {
  origin: HTMLElement;
  content: TemplateRef<any>;
  viewContainerRef: ViewContainerRef;
  data?: T;
  width?: string | number;
  height?: string | number;
};

export type PopoverCloseEvent<T> = {
  type: 'backdropClick' | 'close';
  data?: T;
};

export class PopoverRef<T = any> {
  private _afterClosed = new Subject<PopoverCloseEvent<T>>();
  isOpen: boolean = false;
  afterClosed$ = this._afterClosed.asObservable();

  constructor(public overlay: OverlayRef, public content: PopoverContent, public data?: T) {
    this.isOpen = true;
    overlay.backdropClick().subscribe(() => this._close('backdropClick', data));
  }

  private _close(type: 'backdropClick' | 'close', data?: T) {
    this.isOpen = false;
    this.overlay.dispose();
    this._afterClosed.next({
      type,
      data,
    });
    this._afterClosed.complete();
  }

  close(data?: T) {
    this._close('close', data);
  }
}
