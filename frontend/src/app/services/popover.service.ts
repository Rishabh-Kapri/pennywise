/**
 * Popover service
 * https://netbasal.com/creating-powerful-components-with-angular-cdk-2cef53d81cea
 */

import { ConnectionPositionPair, Overlay, OverlayConfig, PositionStrategy } from '@angular/cdk/overlay';
import { Injectable, Injector } from '@angular/core';
import { PopoverParams, PopoverRef } from './popover-ref';
import { TemplatePortal } from '@angular/cdk/portal';

@Injectable({ providedIn: 'root' })
export class PopoverService {
  constructor(private overlay: Overlay, private injector: Injector) {}

  private getOverlayPosition(origin: HTMLElement): PositionStrategy {
    const positionStrategy = this.overlay
      .position()
      .flexibleConnectedTo(origin)
      .withPositions(this.getPositions())
      .withPush(false);
    return positionStrategy;
  }

  private getPositions(): ConnectionPositionPair[] {
    return [
      {
        originX: 'center',
        originY: 'bottom',
        overlayX: 'center',
        overlayY: 'top',
      },
      {
        originX: 'center',
        originY: 'top',
        overlayX: 'center',
        overlayY: 'bottom',
      },
    ];
  }

  private getOverlayConfig(origin: HTMLElement, width?: string | number, height?: string | number) {
    return new OverlayConfig({
      width,
      height,
      hasBackdrop: true,
      backdropClass: 'popover-backdrop',
      panelClass: 'popover-class',
      positionStrategy: this.getOverlayPosition(origin),
      scrollStrategy: this.overlay.scrollStrategies.reposition(),
    });
  }

  open<T>({ origin, content, data, width, height, viewContainerRef }: PopoverParams<T>) {
    const overlayRef = this.overlay.create(this.getOverlayConfig(origin, width, height));
    const popoverRef = new PopoverRef<T>(overlayRef, content, data);

    // const injector = this.createInjector(popoverRef, this.injector);
    overlayRef.attach(new TemplatePortal(content, viewContainerRef));
    return popoverRef;
  }

  createInjector(popoverRef: PopoverRef, injector: Injector) {
    return Injector.create({
      parent: injector,
      providers: [{ provide: PopoverRef, useValue: popoverRef }],
    });
  }
}
