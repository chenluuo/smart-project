package com.smartagriculture.notification.repository;

import com.smartagriculture.notification.domain.Notification;
import com.smartagriculture.notification.domain.NotificationStatus;
import java.util.List;
import org.springframework.data.jpa.repository.JpaRepository;

public interface NotificationRepository extends JpaRepository<Notification, Long> {

    List<Notification> findByStatus(NotificationStatus status);
}
